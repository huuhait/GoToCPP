package vet

import (
	"cmp"
	"go/ast"
	"go/token"
	"go/types"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/buildssa"
	"golang.org/x/tools/go/analysis/passes/ctrlflow"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/analysis/passes/nilness"
	"golang.org/x/tools/go/analysis/passes/printf"
	"golang.org/x/tools/go/analysis/passes/tests"
	"golang.org/x/tools/go/ast/inspector"
)

// Two analyzers of x/tools crash in a package with C/C++ names, although
// go/types reports no error there: printf (its method finder,
// refactor/satisfy, meets a composite literal of the invalid type) and
// buildssa, which nilness needs (go/ssa has no value for an invalid
// expression). The analyzers below run them on the code that doesn't touch
// such names: the syntax trees they get have the statements (printf) or the
// functions (nilness) with an invalid type left out. Types and facts are
// unchanged, so that the findings of the rest, and the printf facts that a
// package exports (a wrapper of fmt.Sprintf is a wrapper whatever else the
// package does), are the same as in a package without C/C++ names.

// A pruned is the files of a package without the parts that touch invalid
// types.
type pruned struct {
	files []*ast.File
	same  bool // nothing was left out: files is the files of the pass

	// info is the type information to use with files when calls were
	// rewritten (the new nodes have entries in it); nil otherwise.
	info *types.Info

	drops   []token.Pos // what was left out: statements, or functions
	rewrote int         // statements kept with their C/C++ arguments replaced
}

type prunedKey struct {
	info     *types.Info
	stmts    bool
	generics bool // the functions with type parameters are left out too (see pruneGenerics)
}

// The pruned files of a package are made once, as every analyzer of the
// package asks. The driver calls takePruned when it has done with a package,
// which also keeps the cache from growing when vet runs again and again (an
// editor).
var (
	prunedMu sync.Mutex
	prunedBy = make(map[prunedKey]*pruned)
)

// takePruned forgets what prune made for the package with the type
// information info, and returns the notes about it, for the analyzers that
// ran (stmts: printf; else nilness).
func takePruned(info *types.Info, fset *token.FileSet) (notes []prunedNote) {
	prunedMu.Lock()
	defer prunedMu.Unlock()
	for _, k := range []prunedKey{{info, true, false}, {info, false, false}, {info, false, true}} {
		stmts := k.stmts
		p, ok := prunedBy[k]
		if !ok {
			continue
		}
		delete(prunedBy, k)
		if len(p.drops) == 0 && p.rewrote == 0 {
			continue
		}
		n := prunedNote{analyzer: "nilness", rewrote: p.rewrote, dropped: len(p.drops), generics: k.generics}
		if stmts {
			n.analyzer = "printf"
		}
		for _, pos := range p.drops {
			n.where = append(n.where, fset.Position(pos))
		}
		notes = append(notes, n)
	}
	return notes
}

// A prunedNote says what pruning did for an analyzer in a package.
type prunedNote struct {
	analyzer string
	dropped  int // statements (printf) or functions (nilness) left out
	rewrote  int
	where    []token.Position

	// generics: go/ssa crashed on the package, and nilness ran again without its generic functions.
	generics bool
}

func (n prunedNote) text() string {
	var parts []string
	unit := "function"
	if n.analyzer == "printf" {
		unit = "statement"
	}
	if n.generics {
		return plural(n.dropped, "function") + " left out (every generic function, and the ones that use C/C++ values), because go/ssa crashed on one of them: a nil dereference in a generic function is not found"
	}
	if n.dropped > 0 {
		parts = append(parts, plural(n.dropped, unit)+" left out, because they use C/C++ values")
	}
	if n.rewrote > 0 {
		parts = append(parts, plural(n.rewrote, "statement")+" checked with the C/C++ values in calls replaced by unknown ones")
	}
	return strings.Join(parts, "; ")
}

func plural(n int, unit string) string {
	if n == 1 {
		return "1 " + unit
	}
	return strconv.Itoa(n) + " " + unit + "s"
}

// prune returns the files of the pass without what touches invalid types:
// statements if stmts, whole functions otherwise. In the statements that
// stay, the arguments of calls that have C/C++ values are replaced by unknown
// values of the same type (any when the type is not known), so that
// printf still checks the other arguments.
//
// Functions are not rewritten this way: go/ssa builds a value for every
// variable, so a function that declares a variable of a C/C++ type can't be
// built without it. A function that does is left out as a whole (-v says
// which); a function that only calls into C/C++ with plain values is not.
func prune(pass *analysis.Pass, stmts bool) *pruned {
	return pruneWith(pass, stmts, false)
}

// pruneGenerics is prune for functions that also leaves out every function with type parameters
// and every method of a generic type. go/ssa (x/tools v0.44) crashes on some generic code that
// go/types accepts: a conversion of a slice to an array type given by a type parameter
// (func Arr[S ~[]E, A ~[3]E, E any](s S) A { return A(s) }). buildssaGuarded builds again with
// this when the first build panics.
func pruneGenerics(pass *analysis.Pass) *pruned {
	return pruneWith(pass, false, true)
}

func pruneWith(pass *analysis.Pass, stmts, generics bool) *pruned {
	key := prunedKey{pass.TypesInfo, stmts, generics}
	prunedMu.Lock()
	defer prunedMu.Unlock()
	if p, ok := prunedBy[key]; ok {
		return p
	}
	c := &pruner{info: pass.TypesInfo, memo: make(map[types.Type]bool), stmts: stmts, generics: generics}
	p := &pruned{same: true}
	bad := generics
	for _, f := range pass.Files {
		if bad {
			break
		}
		if c.touches(f) {
			bad = true
			break
		}
	}
	if !bad {
		p.files = pass.Files
		prunedBy[key] = p
		return p
	}
	for _, f := range pass.Files {
		nf := c.file(f)
		p.files = append(p.files, nf)
		if nf != f {
			p.same = false
		}
	}
	if p.same {
		p.files = pass.Files
	}
	if c.cloned {
		p.info = c.info
	}
	p.drops, p.rewrote = c.drops, c.rewrote
	prunedBy[key] = p
	return p
}

type pruner struct {
	info     *types.Info
	memo     map[types.Type]bool
	stmts    bool
	generics bool // functions with type parameters are left out too

	cloned  bool // info is a copy that has the entries of the new nodes
	drops   []token.Pos
	rewrote int
}

// touches reports whether a node (nil allowed) has an expression of an
// invalid type.
func (c *pruner) touches(nodes ...ast.Node) bool {
	for _, n := range nodes {
		if n == nil || reflect.ValueOf(n).IsNil() {
			continue
		}
		if touchesInvalid(c.info, c.memo, n) {
			return true
		}
	}
	return false
}

// file returns f, or a copy of it without the declarations or parts that
// touch invalid types.
func (c *pruner) file(f *ast.File) *ast.File {
	var decls []ast.Decl
	changed := false
	for _, d := range f.Decls {
		nd := c.decl(d)
		if nd != d {
			changed = true
		}
		if nd != nil {
			decls = append(decls, nd)
		}
	}
	if !changed {
		return f
	}
	nf := *f
	nf.Decls = decls
	return &nf
}

func (c *pruner) decl(d ast.Decl) ast.Decl {
	switch d := d.(type) {
	case *ast.FuncDecl:
		generic := c.generics && isGenericFunc(d)
		if !generic && !c.touches(d) {
			return d
		}
		nd := *d
		switch {
		case generic || !c.stmts || c.touches(d.Recv, d.Type):
			// Declared without a body, like a function of C: the functions
			// that stay can call it.
			nd.Body = nil
			if d.Body != nil {
				c.drops = append(c.drops, d.Pos())
			}
		default:
			nd.Body = c.block(d.Body)
		}
		return &nd
	case *ast.GenDecl:
		if d.Tok != token.VAR && d.Tok != token.CONST {
			return d
		}
		var specs []ast.Spec
		changed := false
		for _, s := range d.Specs {
			vs, ok := s.(*ast.ValueSpec)
			switch {
			case !c.touches(s) && (c.stmts || !ok || d.Tok != token.VAR || vs.Values == nil):
				specs = append(specs, s)
			case d.Tok == token.VAR && !c.stmts && ok:
				// The functions that stay may use the variable: it stays,
				// typed as go/types typed it, without the value (which may
				// name a function that is gone).
				nv := *vs
				nv.Values = nil
				specs = append(specs, &nv)
				changed = true
			default:
				changed = true
				if c.stmts {
					c.drops = append(c.drops, s.Pos())
				}
			}
		}
		if !changed {
			return d
		}
		if len(specs) == 0 {
			return nil
		}
		nd := *d
		nd.Specs = specs
		return &nd
	}
	return d
}

// isGenericFunc reports whether the declaration has type parameters, or is a method of a generic type.
func isGenericFunc(d *ast.FuncDecl) bool {
	if d.Type.TypeParams != nil && d.Type.TypeParams.NumFields() > 0 {
		return true
	}
	if d.Recv != nil && len(d.Recv.List) > 0 {
		t := d.Recv.List[0].Type
		if s, ok := t.(*ast.StarExpr); ok {
			t = s.X
		}
		switch ast.Unparen(t).(type) {
		case *ast.IndexExpr, *ast.IndexListExpr:
			return true
		}
	}
	return false
}

func (c *pruner) block(b *ast.BlockStmt) *ast.BlockStmt {
	if b == nil {
		return nil
	}
	list, changed := c.list(b.List)
	if !changed {
		return b
	}
	nb := *b
	nb.List = list
	return &nb
}

func (c *pruner) list(in []ast.Stmt) (out []ast.Stmt, changed bool) {
	for _, s := range in {
		ns := c.stmt(s)
		if ns != s {
			changed = true
		}
		if ns != nil {
			out = append(out, ns)
		}
	}
	return out, changed
}

// stmt returns s, a copy of s without the parts of its bodies that touch
// invalid types, or nil if s itself does (its header or, for a simple
// statement, the whole of it).
func (c *pruner) stmt(s ast.Stmt) ast.Stmt {
	if !c.touches(s) {
		return s
	}
	if c.stmts {
		if ns := c.rewritten(s); ns != nil {
			c.rewrote++
			return ns.(ast.Stmt)
		}
	}
	ns := c.pruneStmt(s)
	if ns == nil {
		c.drops = append(c.drops, s.Pos())
	}
	return ns
}

func (c *pruner) pruneStmt(s ast.Stmt) ast.Stmt {
	switch s := s.(type) {
	case *ast.BlockStmt:
		return c.block(s)
	case *ast.LabeledStmt:
		inner := c.stmt(s.Stmt)
		if inner == s.Stmt {
			return s
		}
		if inner == nil {
			inner = &ast.EmptyStmt{Semicolon: s.Colon, Implicit: true}
		}
		n := *s
		n.Stmt = inner
		return &n
	case *ast.IfStmt:
		init, cond := fixed(c, s.Init), fixed(c, s.Cond)
		if c.touches(init, cond) {
			// The condition is out, the statements inside stay: a block
			// with the bodies, which is all printf needs of them.
			bodies := []ast.Stmt{c.block(s.Body)}
			if s.Else != nil {
				bodies = append(bodies, c.stmt(s.Else))
			}
			return c.hoist(s, bodies...)
		}
		n := *s
		n.Init, n.Cond = init, cond
		n.Body = c.block(s.Body)
		if s.Else != nil {
			n.Else = c.stmt(s.Else)
		}
		return &n
	case *ast.ForStmt:
		init, cond, post := fixed(c, s.Init), fixed(c, s.Cond), fixed(c, s.Post)
		if c.touches(init, cond, post) {
			return c.hoist(s, c.block(s.Body))
		}
		n := *s
		n.Init, n.Cond, n.Post = init, cond, post
		n.Body = c.block(s.Body)
		return &n
	case *ast.RangeStmt:
		x := fixed(c, s.X)
		if c.touches(s.Key, s.Value, x) {
			return c.hoist(s, c.block(s.Body))
		}
		n := *s
		n.X = x
		n.Body = c.block(s.Body)
		return &n
	case *ast.SwitchStmt:
		init, tag := fixed(c, s.Init), fixed(c, s.Tag)
		if c.touches(init, tag) {
			return c.hoist(s, c.clauseBodies(s.Body)...)
		}
		n := *s
		n.Init, n.Tag = init, tag
		n.Body = c.clauses(s.Body)
		return &n
	case *ast.TypeSwitchStmt:
		if c.touches(s.Init, s.Assign) {
			return c.hoist(s, c.clauseBodies(s.Body)...)
		}
		n := *s
		n.Body = c.clauses(s.Body)
		return &n
	case *ast.SelectStmt:
		n := *s
		n.Body = c.clauses(s.Body)
		return &n
	}
	return nil
}

// hoist makes the statements of the bodies of s, whose header touches
// invalid types, a block (nil when there is none).
func (c *pruner) hoist(s ast.Stmt, bodies ...ast.Stmt) ast.Stmt {
	b := &ast.BlockStmt{Lbrace: s.Pos(), Rbrace: s.End() - 1}
	for _, body := range bodies {
		if body != nil && !reflect.ValueOf(body).IsNil() {
			b.List = append(b.List, body)
		}
	}
	if len(b.List) == 0 {
		return nil // stmt records it
	}
	c.drops = append(c.drops, s.Pos()) // its header
	return b
}

// clauseBodies returns the bodies of the cases of a switch, as blocks, with
// what touches invalid types left out.
func (c *pruner) clauseBodies(b *ast.BlockStmt) []ast.Stmt {
	var out []ast.Stmt
	for _, s := range b.List {
		if cl, ok := s.(*ast.CaseClause); ok {
			body, _ := c.list(cl.Body)
			out = append(out, &ast.BlockStmt{List: body})
		}
	}
	return out
}

// clauses prunes the case clauses of a switch or select.
func (c *pruner) clauses(b *ast.BlockStmt) *ast.BlockStmt {
	nb := *b
	nb.List = nil
	for _, s := range b.List {
		switch cl := s.(type) {
		case *ast.CaseClause:
			if c.touches(exprNodes(cl.List)...) {
				c.drops = append(c.drops, cl.Pos())
				continue
			}
			n := *cl
			n.Body, _ = c.list(cl.Body)
			nb.List = append(nb.List, &n)
		case *ast.CommClause:
			if c.touches(cl.Comm) {
				c.drops = append(c.drops, cl.Pos())
				continue
			}
			n := *cl
			n.Body, _ = c.list(cl.Body)
			nb.List = append(nb.List, &n)
		}
	}
	return &nb
}

func exprNodes(es []ast.Expr) []ast.Node {
	out := make([]ast.Node, len(es))
	for i, e := range es {
		out[i] = e
	}
	return out
}

// printfGuarded is printf that also runs in a package with C/C++ names,
// on the statements that don't touch them.
var printfGuarded = func() *analysis.Analyzer {
	g := *printf.Analyzer
	g.Run = func(pass *analysis.Pass) (any, error) {
		p := prune(pass, true)
		if p.same {
			return printf.Analyzer.Run(pass)
		}
		np := *pass
		np.Files = p.files
		if p.info != nil {
			np.TypesInfo = p.info
		}
		np.ResultOf = map[*analysis.Analyzer]any{inspect.Analyzer: inspector.New(p.files)}
		return printf.Analyzer.Run(&np)
	}
	return &g
}()

// buildssaGuarded builds the SSA form of the functions that don't touch
// invalid types.
var buildssaGuarded = func() *analysis.Analyzer {
	g := *buildssa.Analyzer
	g.Run = func(pass *analysis.Pass) (any, error) {
		res, crashed, err := tryRun(func() (any, error) { return buildSSA(pass, prune(pass, false)) })
		if crashed == nil {
			return res, err
		}
		// go/ssa crashes on some generic code that go/types accepts (see pruneGenerics): build again
		// without the generic functions. A second crash is a real bug and reaches the driver.
		return buildSSA(pass, pruneGenerics(pass))
	}
	return &g
}()

// buildSSA builds the SSA form of the files of p.
func buildSSA(pass *analysis.Pass, p *pruned) (any, error) {
	if p.same {
		return buildssa.Analyzer.Run(pass)
	}
	np := *pass
	np.Files = p.files
	// go/ssa builds the initialization of the package from InitOrder,
	// the initializers that go/types found, not from the files; the
	// variables that were stripped have none.
	info := *pass.TypesInfo
	if p.info != nil {
		info = *p.info
	}
	info.InitOrder = nil
	np.TypesInfo = &info
	return buildssa.Analyzer.Run(&np)
}

// tryRun runs f and returns what it panicked with, if it did.
func tryRun(f func() (any, error)) (res any, crashed any, err error) {
	defer func() {
		if r := recover(); r != nil {
			crashed = r
		}
	}()
	res, err = f()
	return res, nil, err
}

// nilnessGuarded is nilness over buildssaGuarded. nilness itself crashes on some generic code too (a
// slice converted to an array type of a type parameter, x/tools v0.44, nilness.go slice2ArrayPtrLen): it
// runs again over an SSA form without the generic functions (see pruneGenerics), and -v says so.
var nilnessGuarded = func() *analysis.Analyzer {
	g := *nilness.Analyzer
	// ctrlflow is what buildssa needs: the second build, after a crash, runs from this pass.
	g.Requires = []*analysis.Analyzer{buildssaGuarded, ctrlflow.Analyzer}
	g.Run = func(pass *analysis.Pass) (any, error) {
		run := func(ssa any) (any, error) {
			np := *pass
			np.ResultOf = map[*analysis.Analyzer]any{buildssa.Analyzer: ssa}
			return nilness.Analyzer.Run(&np)
		}
		// Findings are reported as nilness goes; those of a run that crashed are repeated by the second
		// run, and the driver drops repeats of a finding.
		res, crashed, err := tryRun(func() (any, error) { return run(pass.ResultOf[buildssaGuarded]) })
		if crashed == nil {
			return res, err
		}
		bp := *pass
		bp.ResultOf = map[*analysis.Analyzer]any{ctrlflow.Analyzer: pass.ResultOf[ctrlflow.Analyzer]}
		ssa, err := buildSSA(&bp, pruneGenerics(pass))
		if err != nil {
			return nil, err
		}
		return run(ssa)
	}
	return &g
}()

// testsGuarded is the tests analyzer of x/tools, which only looks at files
// named *_test.go: the files of gocpp are *_test.gp, so it gets a copy of
// the file set where they are named so. The positions are the same (the
// bases and the lines are copied), so the findings are at the .gp files.
var testsGuarded = func() *analysis.Analyzer {
	g := *tests.Analyzer
	g.Run = func(pass *analysis.Pass) (any, error) {
		files := slices.Clone(pass.Files)
		slices.SortFunc(files, func(a, b *ast.File) int { return cmp.Compare(a.FileStart, b.FileStart) })
		fset := token.NewFileSet()
		for _, f := range files {
			tf := pass.Fset.File(f.FileStart)
			name := tf.Name()
			if strings.HasSuffix(name, "_test.gp") {
				name = strings.TrimSuffix(name, ".gp") + ".go"
			}
			nf := fset.AddFile(name, tf.Base(), tf.Size())
			nf.SetLines(tf.Lines())
		}
		np := *pass
		np.Fset = fset
		return tests.Analyzer.Run(&np)
	}
	return &g
}()
