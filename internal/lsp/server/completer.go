package server

import (
	"context"
	"go/ast"
	"go/token"
	"go/types"
	"math"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/tools/go/ast/astutil"

	"github.com/huuhait/gocpp/internal/load"
	"github.com/huuhait/gocpp/internal/lsp/cache"
	"github.com/huuhait/gocpp/internal/lsp/protocol"
)

// The candidates of textDocument/completion (step 11.6, completion.go), a
// smaller version of gopls's completer (see COPIED.md): the names in scope
// at the cursor, the fields and methods after x., the members of a package
// after pkg. (also of a package to import, autoimport.go), the field names
// of a struct literal, and the keywords where a statement or a declaration
// starts. gopls's deep completion (x.f.g), postfix snippets and literal
// candidates (T{}) are not there.
//
// The candidates come from the checked package of the document, also when
// the file doesn't parse (check.Package.Partial): x. at the end of a line
// parses as x._ (the parser's "phantom" selector) or as x.next, with the
// name at the start of the next line, and in both cases go/types has the
// type of x. The name being typed is read from the text itself (the
// letters around the cursor), and the selector from the expression that
// ends at the dot before it. When the parser took the next line into a
// phantom selector (x. then var y = 1), the package is checked again for
// completion with x._ in the text (fixDangling in completion.go, like
// gopls's fixDanglingSelector), so the names of that line are there too.
//
// Differences from gopls besides the missing parts: the keywords come at a
// blank statement too, not only while a name is being typed; the prefix
// filter is a prefix in any case, not gopls's fuzzy match; the
// documentation comes from completionItem/resolve.

// The scores of the candidates, as gopls has them: a candidate that fits
// the expected type gets highScore times its score.
const (
	stdScore  = 1.0
	highScore = 10.0
	// unimportedScore is the score of a package to import, and of its
	// members (gopls's unimportedScore with no relevance).
	unimportedScore = stdScore / 2
)

// A candidate is a completion candidate before it is formatted.
type candidate struct {
	obj     types.Object // nil for a keyword or a header package
	name    string       // the label
	kind    protocol.CompletionItemKind
	detail  string // the detail, when obj is nil
	score   float64
	invoke  bool // a call: Join(…)
	field   bool // a field name in a struct literal: Name: …
	keyword bool
	insert  string // the text to insert, when it is not the name ("return ")
	// exp is the API of a package to import, which obj comes from;
	// imp is the package to import, also for a package name.
	exp *cache.Exports
	imp *importable
}

// A completer collects the candidates of one request.
type completer struct {
	q   *query
	ctx context.Context
	pf  *cache.ParsedFile // the document as the user wrote it
	ix  *importIndex
	// filter: the candidates must start with the prefix (not for
	// completionItem/resolve, which looks for the candidate it had).
	filter bool

	off        int // the cursor, as an offset in q.src
	start, end int // the name around the cursor: q.src[start:end]
	prefix     string
	pos        token.Pos  // the start of the name (the cursor when there is none)
	path       []ast.Node // the checked nodes around the cursor, innermost first
	qual       types.Qualifier

	expected types.Type // the type that the place wants, or nil
	wantType bool       // the place wants a type: var x T
	lit      *compLit   // a struct literal whose field names the place may want

	importable []importable // the packages to import (importables), once
	cands      []*candidate
	seen       map[types.Object]bool
	names      map[string]bool                // the names in scope that are offered (shadowing)
	imports    map[string][]protocol.TextEdit // the edits that add an import, by path (importEdit)
}

// A compLit is the struct literal around the cursor (gopls's compLitInfo).
type compLit struct {
	lit      *ast.CompositeLit
	st       *types.Struct
	kv       *ast.KeyValueExpr // the key: value that the cursor is in, or nil
	inKey    bool              // the cursor is in a field name for sure
	maybeKey bool              // a field name or a value: T{|} or T{Na|}
}

// newCompleter returns the completer at the place of q, or nil where
// nothing is offered: in a comment, a literal or an import declaration.
// pf is the document as the user wrote it.
func newCompleter(ctx context.Context, q *query, pf *cache.ParsedFile, ix *importIndex, filter bool) (*completer, error) {
	if pf.Tok.Size() != len(q.src) {
		return nil, nil // can't be: the same snapshot
	}
	c := &completer{q: q, ctx: ctx, pf: pf, ix: ix, filter: filter,
		off: q.tok.Offset(q.pos), seen: make(map[types.Object]bool), names: make(map[string]bool)}
	if c.noCompletion() {
		return nil, nil
	}
	c.start, c.end = nameAround(q.src, c.off)
	if c.start < c.end {
		if r, _ := utf8.DecodeRune(q.src[c.start:]); unicode.IsDigit(r) {
			return nil, nil // a number
		}
	}
	c.prefix = string(q.src[c.start:c.off])
	c.pos = q.tok.Pos(c.start)
	c.qual = q.qualifier()
	return c, nil
}

// nameAround returns the start and the end of the name (letters, digits and
// _) around the offset off of src.
func nameAround(src []byte, off int) (start, end int) {
	start, end = off, off
	for start > 0 {
		r, n := utf8.DecodeLastRune(src[:start])
		if !isNameRune(r) {
			break
		}
		start -= n
	}
	for end < len(src) {
		r, n := utf8.DecodeRune(src[end:])
		if !isNameRune(r) {
			break
		}
		end += n
	}
	return start, end
}

func isNameRune(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

// noCompletion reports whether the cursor is where nothing is completed: in
// a comment, a string, character or number literal (gopls: also right
// after one), or in an import declaration (gopls completes import paths
// there; not yet).
func (c *completer) noCompletion() bool {
	pf := c.pf
	pos := pf.Tok.Pos(c.off)
	for _, cg := range pf.File.Comments {
		for _, cm := range cg.List {
			if cm.Pos() < pos && (pos < cm.End() || pos == cm.End() && strings.HasPrefix(cm.Text, "//")) {
				return true
			}
		}
	}
	for _, decl := range pf.File.Decls {
		if gd, ok := decl.(*ast.GenDecl); ok && gd.Tok == token.IMPORT && gd.Pos() < pos && pos <= gd.End() {
			return true
		}
	}
	if c.off == 0 {
		return false
	}
	path, _ := astutil.PathEnclosingInterval(pf.File, pos-1, pos-1)
	if len(path) > 0 {
		if _, ok := path[0].(*ast.BasicLit); ok {
			return true
		}
	}
	return false
}

// collect collects the candidates.
func (c *completer) collect() {
	q := c.q
	if !c.pf.File.Package.IsValid() {
		// A new file: it needs its package clause first.
		c.addKeywords(stdScore, "package")
		return
	}
	at := c.pos
	if c.off > 0 {
		at = q.tok.Pos(c.off - 1) // what is before the cursor (gopls)
	}
	c.path, _ = astutil.PathEnclosingInterval(q.file.AST, at, at)
	if dot := c.start - 1; dot >= 0 && q.src[dot] == '.' {
		// The members that fit the place of x.f come first too:
		// helper(p.|) wants an int.
		c.expected, _ = c.expectedType()
		c.selector(q.tok.Pos(dot))
		return
	}
	if len(c.path) == 0 {
		return
	}
	if id, ok := c.path[0].(*ast.Ident); ok && id.Pos() <= c.pos && c.pos <= id.End() {
		// No completion of a name that is being declared (gopls), but
		// of an embedded field, which names a type.
		if obj := q.info.Defs[id]; obj != nil {
			if v, ok := obj.(*types.Var); !ok || !v.Embedded() {
				return
			}
		}
	}
	switch c.path[0].(type) {
	case *ast.File, *ast.BadDecl:
		// The top level of the file: only the declaration keywords
		// (gopls: a name there is a BadDecl, never an *ast.Ident).
		c.addKeywords(stdScore, declKeywords...)
		return
	}
	c.expected, c.wantType = c.expectedType()
	c.lit = c.structLit()
	if c.lit != nil && (c.lit.inKey || c.lit.maybeKey) {
		c.structFields()
		if !c.lit.maybeKey {
			return
		}
	}
	c.lexical()
	c.keywords()
}

// match reports whether the name fits the prefix: a prefix of it, in any
// case.
func (c *completer) match(name string) bool {
	return !c.filter || len(name) >= len(c.prefix) && strings.EqualFold(name[:len(c.prefix)], c.prefix)
}

// add adds the object as a candidate with the score, unless it was added
// already, or is an unexported object of another package, or doesn't fit
// the prefix.
func (c *completer) add(obj types.Object, score float64, exp *cache.Exports, imp *importable) *candidate {
	if c.seen[obj] || obj.Name() == "_" || obj.Name() == "" {
		return nil
	}
	if obj.Pkg() != nil && obj.Pkg() != c.q.types && !obj.Exported() {
		return nil
	}
	if !c.match(obj.Name()) {
		return nil
	}
	c.seen[obj] = true
	cand := &candidate{obj: obj, name: obj.Name(), score: score, exp: exp, imp: imp}
	c.rank(cand)
	c.cands = append(c.cands, cand)
	return cand
}

// rank sets the score of cand from what the place wants (gopls's
// addCandidate and matchingCandidate, without the deep candidates), and
// whether a function is called.
func (c *completer) rank(cand *candidate) {
	obj := cand.obj
	_, isType := obj.(*types.TypeName)
	_, isPkg := obj.(*types.PkgName)
	switch {
	case c.wantType:
		if isType || isPkg {
			cand.score *= highScore
		}
	case isType || isPkg:
	case c.expected != nil && fits(obj.Type(), c.expected):
		cand.score *= highScore
		if b, ok := obj.Type().(*types.Basic); ok && b.Info()&types.IsUntyped != 0 &&
			!types.Identical(b, c.expected) && obj.Parent() != types.Universe {
			// An untyped constant that converts: after the values of
			// the type itself (gopls).
			cand.score *= 0.75
		}
	default:
		if sig, ok := obj.Type().Underlying().(*types.Signature); ok && c.expected != nil &&
			sig.Results().Len() == 1 && fits(sig.Results().At(0).Type(), c.expected) {
			cand.score *= highScore
			cand.invoke = true
		}
		if _, ok := obj.(*types.Func); ok {
			cand.invoke = true // a function is called by default
		}
	}
	if _, ok := obj.(*types.Builtin); ok && !c.wantType {
		cand.invoke = true
	}
	if fn, ok := obj.(*types.Func); ok && cand.invoke {
		if sig, ok := fn.Type().(*types.Signature); ok && sig.Recv() != nil {
			cand.score *= 0.9 // fields and variables before method calls
		}
	}
	if !obj.Exported() && obj.Parent() != types.Universe {
		cand.score *= 1.1 // the package's own names first
	}
}

// fits reports whether a value of the type t can be used where the type
// want is expected.
func fits(t, want types.Type) bool {
	if t == nil || want == nil || cache.Incomplete(t) || cache.Incomplete(want) {
		return false
	}
	return types.AssignableTo(t, want)
}

// selector collects the candidates after the dot at dot: the fields and
// methods of x in x., or the members of the package pkg in pkg.
func (c *completer) selector(dot token.Pos) {
	q := c.q
	// The expression that ends at the dot: x in x._ (a phantom name), in
	// x.next (the name on the next line) or in x.Na (being typed), or in
	// x.(T) before its type is typed.
	var x ast.Expr
	ast.Inspect(q.file.AST, func(n ast.Node) bool {
		if n == nil || n.Pos() > dot || dot >= n.End() {
			return false
		}
		switch n := n.(type) {
		case *ast.SelectorExpr:
			if n.X.End() <= dot && dot < n.Sel.Pos() {
				x = n.X
			}
		case *ast.TypeAssertExpr:
			if n.X.End() <= dot && dot < n.Lparen {
				x = n.X
			}
		}
		return true
	})
	if x == nil {
		// Not in the tree (broken code): a name before the dot.
		start, end := nameAround(q.src, q.tok.Offset(dot))
		if start == end || end != q.tok.Offset(dot) {
			return
		}
		c.qualified(string(q.src[start:end]), q.tok.Pos(start))
		return
	}
	if c.cpp(x) {
		// A header package, a namespace, C or a C/C++ value: what they
		// have comes from the headers, with the C/C++ completion of
		// Phase 12. The checker knows only that they are opaque. But a
		// name that Go declares nowhere is taken as a namespace in a file
		// with header imports (strings. without import "strings"): the
		// packages to import with that name come first.
		if name := c.namespaceGuess(x); name != "" {
			c.unimportedMembers(name)
		}
		return
	}
	if id, ok := x.(*ast.Ident); ok {
		// A package name, or a name declared nowhere (a package to
		// import): go/types has an invalid type for it.
		switch q.info.ObjectOf(id).(type) {
		case *types.PkgName, nil:
			c.qualified(id.Name, id.Pos())
			return
		}
	}
	if tv, ok := q.info.Types[x]; ok && tv.Type != nil && tv.Type != types.Typ[types.Invalid] {
		c.methodsAndFields(tv.Type, tv.Addressable())
	}
}

// cpp reports whether x is C/C++: a header package or a namespace (or a
// selector on one), C, or a value whose type comes from C/C++.
func (c *completer) cpp(x ast.Expr) bool {
	q := c.q
	left := x
	for {
		if sel, ok := left.(*ast.SelectorExpr); ok {
			left = sel.X
			continue
		}
		break
	}
	if id, ok := left.(*ast.Ident); ok {
		if rw := q.file.Rewrite; rw != nil {
			for _, u := range rw.Uses {
				if u.Ident.Pos() == id.Pos() {
					return true
				}
			}
		}
		if pn, ok := q.info.Uses[id].(*types.PkgName); ok && pn.Imported().Path() == "C" {
			return true
		}
	}
	return q.opaque.IsOpaque(x)
}

// namespaceGuess returns the name of x when x is a name that the loader
// took as a C++ namespace only because Go declares it nowhere (not a
// namespace rename), or "".
func (c *completer) namespaceGuess(x ast.Expr) string {
	rw, info := c.q.file.Rewrite, c.q.file.Info
	sel, ok := x.(*ast.SelectorExpr) // C.strings, after the rewrite
	if rw == nil || !ok {
		return ""
	}
	for _, u := range rw.Uses {
		if u.Kind == load.NamespaceUse && u.Rename == nil && !u.Bare && sel.Sel == u.Ident &&
			(info == nil || info.NamespaceRenames[u.Ident.Name] == nil) {
			return u.Ident.Name
		}
	}
	return ""
}

// qualified collects the members of the package that the name, used at
// pos, names in pkg.: an import of the file, or else, when the name is
// declared nowhere, a package to import.
func (c *completer) qualified(name string, pos token.Pos) {
	q := c.q
	scope := q.types.Scope().Innermost(pos)
	if scope == nil {
		scope = q.info.Scopes[q.file.AST]
	}
	var obj types.Object
	if scope != nil {
		_, obj = scope.LookupParent(name, pos)
	}
	if pn, ok := obj.(*types.PkgName); ok {
		if pn.Imported().Path() != "C" {
			c.packageMembers(pn.Imported(), stdScore, nil, nil)
		}
		return
	}
	if obj == nil {
		c.unimportedMembers(name)
	}
}

// maxUnimported is the number of packages with the same name whose
// members are offered (gopls's maxUnimportedPackageNames).
const maxUnimported = 5

// importables returns the packages that the document can import, made
// once per request.
func (c *completer) importables() []importable {
	if c.importable == nil {
		c.importable = c.ix.importables(c.q)
	}
	return c.importable
}

// unimportedMembers collects the members of the packages to import that
// are called name.
func (c *completer) unimportedMembers(name string) {
	if c.ix == nil {
		return
	}
	n := 0
	for _, p := range c.importables() {
		if p.name != name || n == maxUnimported {
			continue
		}
		n++
		exp, err := c.q.snap.Exports(c.ctx, p.dir, p.std, p.path)
		if err != nil || exp == nil {
			continue
		}
		c.packageMembers(exp.Types, unimportedScore, exp, &p)
	}
}

// packageMembers collects the members of the package pkg.
func (c *completer) packageMembers(pkg *types.Package, score float64, exp *cache.Exports, imp *importable) {
	scope := pkg.Scope()
	for _, name := range scope.Names() {
		c.add(scope.Lookup(name), score, exp, imp)
	}
}

// methodsAndFields collects the methods and the fields of a value of the
// type t (gopls's methodsAndFields): of an addressable value of a type
// that is no pointer or interface, the methods of *T too.
func (c *completer) methodsAndFields(t types.Type, addressable bool) {
	var mset *types.MethodSet
	if _, isPtr := t.Underlying().(*types.Pointer); addressable && !types.IsInterface(t) && !isPtr {
		mset = types.NewMethodSet(types.NewPointer(t))
	} else {
		mset = types.NewMethodSet(t)
	}
	for m := range mset.Methods() {
		c.add(m.Obj(), stdScore, nil, nil)
	}
	eachField(t, func(v *types.Var) { c.add(v, stdScore-0.01, nil, nil) })
}

// eachField calls fn for each field of the struct t (or *t), also the
// promoted ones, breadth first (gopls's eachField).
func eachField(t types.Type, fn func(*types.Var)) {
	seen := make(map[*types.Struct]bool)
	var visit func(t types.Type)
	visit = func(t types.Type) {
		if p, ok := t.Underlying().(*types.Pointer); ok {
			t = p.Elem()
		}
		st, ok := t.Underlying().(*types.Struct)
		if !ok || seen[st] {
			return
		}
		seen[st] = true
		var embedded []types.Type
		for f := range st.Fields() {
			fn(f)
			if f.Embedded() {
				embedded = append(embedded, f.Type())
			}
		}
		for _, e := range embedded {
			visit(e)
		}
	}
	visit(t)
}

// lexical collects the names in scope at the cursor (gopls's lexical): the
// innermost scope first, and a name only once (the innermost one shadows
// the others), then the header packages of the file and the packages to
// import.
func (c *completer) lexical() {
	q := c.q
	var scopes []*types.Scope
	for _, n := range c.path {
		switch x := n.(type) {
		case *ast.FuncDecl:
			n = x.Type
		case *ast.FuncLit:
			n = x.Type
		}
		if s := q.info.Scopes[n]; s != nil {
			scopes = append(scopes, s)
		}
	}
	scopes = append(scopes, q.types.Scope(), types.Universe)
	inConst := c.inConst()
	for i, scope := range scopes {
		for _, name := range scope.Names() {
			declScope, obj := scope.LookupParent(name, c.pos)
			if declScope != scope || c.names[name] {
				continue // declared after the cursor, or shadowed
			}
			if pn, ok := obj.(*types.PkgName); ok && pn.Imported().Path() == "C" {
				continue // the import "C" that the loader adds
			}
			if name == "iota" && obj.Parent() == types.Universe && !inConst {
				continue
			}
			score := stdScore * math.Pow(.99, float64(i))
			if _, ok := obj.(*types.Nil); ok {
				score /= 2
			}
			if cand := c.add(obj, score, nil, nil); cand != nil {
				c.names[name] = true
			}
		}
	}
	// The header packages and namespace renames of the file: names that
	// go/types never sees.
	if info := q.file.Info; info != nil {
		for name, inc := range info.HeaderNames {
			c.addName(name, `"`+inc.Path+`" (C/C++ header)`, stdScore)
		}
		for name, rn := range info.NamespaceRenames {
			c.addName(name, `"`+rn.Namespace+`" (C++ namespace)`, stdScore)
		}
	}
	c.unimportedPackages()
}

// addName adds a candidate without an object: a header package name.
func (c *completer) addName(name, detail string, score float64) {
	if c.names[name] || !c.match(name) {
		return
	}
	c.names[name] = true
	if c.wantType {
		score *= highScore // its types: rl.Color
	}
	c.cands = append(c.cands, &candidate{name: name, kind: protocol.ModuleCompletion, detail: detail, score: score})
}

// unimportedPackages collects the packages to import whose names start
// with the prefix (gopls's unimportedPackages), when there is one.
func (c *completer) unimportedPackages() {
	if c.ix == nil || c.prefix == "" && c.filter {
		return
	}
	n := 0
	for _, p := range c.importables() {
		if c.names[p.name] || !c.match(p.name) || n == maxUnimported {
			continue
		}
		n++
		c.names[p.name] = true
		imp := p
		c.cands = append(c.cands, &candidate{name: p.name, kind: protocol.ModuleCompletion,
			detail: `"` + p.path + `"`, score: unimportedScore, imp: &imp})
	}
}

// inConst reports whether the cursor is in a const declaration (iota).
func (c *completer) inConst() bool {
	for _, n := range c.path {
		if gd, ok := n.(*ast.GenDecl); ok && gd.Tok == token.CONST {
			return true
		}
	}
	return false
}

// structLit returns the struct literal around the cursor whose field
// names the place may want, or nil (gopls's enclosingCompositeLiteral).
func (c *completer) structLit() *compLit {
	pos := c.pos
	for _, n := range c.path {
		switch n := n.(type) {
		case *ast.CompositeLit:
			if !(n.Lbrace < pos && (pos <= n.Rbrace || !n.Rbrace.IsValid())) {
				continue // in its type: Foo|{}
			}
			t := c.q.info.TypeOf(n)
			if t == nil {
				return nil
			}
			if p, ok := t.Underlying().(*types.Pointer); ok {
				t = p.Elem()
			}
			st, ok := t.Underlying().(*types.Struct)
			if !ok {
				return nil
			}
			cl := &compLit{lit: n, st: st}
			var at ast.Expr
			keys := false
			for _, el := range n.Elts {
				if el.Pos() <= pos && pos <= el.End() {
					at = el
				}
				if kv, ok := el.(*ast.KeyValueExpr); ok {
					keys = true
					if at == el {
						cl.kv = kv
					}
				}
			}
			switch {
			case cl.kv != nil:
				cl.inKey = pos <= cl.kv.Colon
			case keys:
				cl.inKey = true
			case len(n.Elts) == 0:
				cl.maybeKey = true
			case len(n.Elts) == 1:
				_, cl.maybeKey = at.(*ast.Ident)
			}
			return cl
		case *ast.CallExpr:
			if !c.q.contains(n.Fun) {
				return nil
			}
		case *ast.FuncLit, *ast.IndexExpr, *ast.SliceExpr, ast.Stmt:
			return nil
		}
	}
	return nil
}

// structFields collects the fields of the struct literal that are not set
// yet, in their order (gopls's structLiteralFieldName).
func (c *completer) structFields() {
	set := make(map[types.Object]bool)
	for _, el := range c.lit.lit.Elts {
		if kv, ok := el.(*ast.KeyValueExpr); ok && kv != c.lit.kv {
			if id, ok := kv.Key.(*ast.Ident); ok {
				set[c.q.info.Uses[id]] = true
			}
		}
	}
	for i := range c.lit.st.NumFields() {
		f := c.lit.st.Field(i)
		if set[f] || c.seen[f] || f.Pkg() != c.q.types && !f.Exported() || !c.match(f.Name()) {
			continue
		}
		c.seen[f] = true
		c.cands = append(c.cands, &candidate{obj: f, name: f.Name(), score: highScore - float64(i)*0.0001, field: true})
	}
}

// expectedType returns the type that the place of the cursor wants
// (gopls's expectedCandidate, for the cheap cases): the right side of an
// assignment, a variable's value, a result of return, an argument of a
// call or a conversion, a field or element value in a composite literal,
// the other side of a comparison or an operation, a value to send; and
// whether the place wants a type.
func (c *completer) expectedType() (types.Type, bool) {
	t, wantType := c.expectedType1()
	if t != nil {
		if iface, ok := t.Underlying().(*types.Interface); ok && iface.Empty() {
			// Anything fits any: no expected type (gopls's
			// candTypeMatches skips the empty interface).
			t = nil
		}
	}
	return t, wantType
}

// expectedType1 is expectedType, also with an empty interface.
func (c *completer) expectedType1() (types.Type, bool) {
	q, pos := c.q, c.pos
	typeOf := func(e ast.Expr) types.Type {
		t := q.info.TypeOf(e)
		if t == nil || t == types.Typ[types.Invalid] {
			return nil
		}
		return t
	}
	for i, n := range c.path {
		switch n := n.(type) {
		case *ast.AssignStmt:
			if pos <= n.TokPos || n.Tok == token.DEFINE {
				return nil, false
			}
			j := exprIndex(n.Rhs, pos)
			if len(n.Lhs) == len(n.Rhs) || len(n.Rhs) <= 1 && len(n.Lhs) == 1 {
				if j < len(n.Lhs) {
					return typeOf(n.Lhs[j]), false
				}
			}
			return nil, false
		case *ast.ValueSpec:
			if n.Type != nil && q.contains(n.Type) {
				return nil, true
			}
			if n.Type != nil && pos > n.Type.End() {
				return typeOf(n.Type), false
			}
			return nil, n.Type == nil && len(n.Values) == 0 && len(n.Names) > 0 && pos > n.Names[len(n.Names)-1].End()
		case *ast.Field:
			return nil, n.Type != nil && q.contains(n.Type)
		case *ast.TypeSpec:
			return nil, n.Type != nil && q.contains(n.Type)
		case *ast.StarExpr, *ast.ArrayType, *ast.MapType, *ast.ChanType, *ast.Ellipsis:
			// In a type? Its parent says.
			if i+1 < len(c.path) {
				switch p := c.path[i+1].(type) {
				case *ast.Field, *ast.ValueSpec, *ast.TypeSpec, *ast.StarExpr, *ast.ArrayType, *ast.MapType, *ast.ChanType:
					continue
				case *ast.CompositeLit:
					if p.Type == n {
						return nil, true
					}
				}
			}
			return nil, false
		case *ast.ReturnStmt:
			sig := c.enclosingSignature()
			if sig == nil {
				return nil, false
			}
			if j := exprIndex(n.Results, pos); j < sig.Results().Len() {
				return sig.Results().At(j).Type(), false
			}
			return nil, false
		case *ast.CallExpr:
			if !(n.Lparen < pos && (pos <= n.Rparen || !n.Rparen.IsValid())) {
				if q.contains(n.Fun) {
					continue // the function's name: Fo|(x)
				}
				return nil, false
			}
			tv, ok := q.info.Types[n.Fun]
			if !ok || tv.Type == nil {
				return nil, false
			}
			if tv.IsType() {
				return tv.Type, false // a conversion
			}
			sig, ok := tv.Type.Underlying().(*types.Signature)
			if !ok {
				return nil, false
			}
			j, np := exprIndex(n.Args, pos), sig.Params().Len()
			switch {
			case sig.Variadic() && j >= np-1 && !n.Ellipsis.IsValid():
				if s, ok := sig.Params().At(np - 1).Type().(*types.Slice); ok {
					return s.Elem(), false
				}
			case j < np:
				return sig.Params().At(j).Type(), false
			}
			return nil, false
		case *ast.KeyValueExpr:
			if pos <= n.Colon {
				// A key: the struct literal says (structLit); the
				// key of a map wants the key type.
				if i+1 < len(c.path) {
					if cl, ok := c.path[i+1].(*ast.CompositeLit); ok {
						if m, ok := underOf(typeOf(cl)).(*types.Map); ok {
							return m.Key(), false
						}
					}
				}
				return nil, false
			}
			if i+1 < len(c.path) {
				if cl, ok := c.path[i+1].(*ast.CompositeLit); ok {
					switch t := underOf(typeOf(cl)).(type) {
					case *types.Struct:
						if id, ok := n.Key.(*ast.Ident); ok {
							if v, ok := q.info.Uses[id].(*types.Var); ok {
								return v.Type(), false
							}
						}
					case *types.Map:
						return t.Elem(), false
					case *types.Slice:
						return t.Elem(), false
					case *types.Array:
						return t.Elem(), false
					}
				}
			}
			return nil, false
		case *ast.CompositeLit:
			if !(n.Lbrace < pos && (pos <= n.Rbrace || !n.Rbrace.IsValid())) {
				if n.Type != nil && q.contains(n.Type) {
					return nil, true
				}
				continue
			}
			switch t := underOf(typeOf(n)).(type) {
			case *types.Slice:
				return t.Elem(), false
			case *types.Array:
				return t.Elem(), false
			case *types.Map:
				return t.Key(), false
			case *types.Struct:
				// Without keys, the fields in order.
				if j := exprIndex(n.Elts, pos); j < t.NumFields() {
					return t.Field(j).Type(), false
				}
			}
			return nil, false
		case *ast.BinaryExpr:
			if pos <= n.OpPos {
				continue
			}
			switch n.Op {
			case token.LAND, token.LOR:
				return types.Typ[types.Bool], false
			case token.SHL, token.SHR:
				return nil, false
			}
			t := typeOf(n.X)
			if b, ok := t.(*types.Basic); ok && b.Info()&types.IsUntyped != 0 {
				return nil, false
			}
			return t, false
		case *ast.SendStmt:
			if pos > n.Arrow {
				if ch, ok := underOf(typeOf(n.Chan)).(*types.Chan); ok {
					return ch.Elem(), false
				}
			}
			return nil, false
		case *ast.FuncLit, *ast.IndexExpr, *ast.SliceExpr, *ast.BlockStmt, ast.Stmt, ast.Decl:
			return nil, false
		}
	}
	return nil, false
}

// underOf returns the underlying type of t (through a pointer), or nil.
func underOf(t types.Type) types.Type {
	if t == nil {
		return nil
	}
	if p, ok := t.Underlying().(*types.Pointer); ok {
		t = p.Elem()
	}
	return t.Underlying()
}

// exprIndex returns the index in list of the expression at pos: the number
// of expressions that end before it.
func exprIndex(list []ast.Expr, pos token.Pos) int {
	i := 0
	for _, e := range list {
		if pos <= e.End() {
			break
		}
		i++
	}
	return i
}

// enclosingSignature returns the signature of the function around the
// cursor, or nil.
func (c *completer) enclosingSignature() *types.Signature {
	for _, n := range c.path {
		switch n := n.(type) {
		case *ast.FuncDecl:
			if fn, ok := c.q.info.Defs[n.Name].(*types.Func); ok {
				sig, _ := fn.Type().(*types.Signature)
				return sig
			}
			return nil
		case *ast.FuncLit:
			sig, _ := c.q.info.TypeOf(n).(*types.Signature)
			return sig
		}
	}
	return nil
}

// The keywords (gopls's keywords.go).
var (
	declKeywords = []string{"const", "func", "import", "type", "var"}
	stmtKeywords = []string{"defer", "return", "for", "go", "switch", "select", "if", "else", "var", "const", "goto", "type"}
)

// keywords collects the keywords that can come at the cursor (gopls's
// addKeywordCompletions): the declaration keywords at the top level of the
// file, the statement keywords where a statement starts, with break,
// continue, case, default and fallthrough where they make sense. They are
// Go's keywords; the gates of the gocpp phases explain what a phase
// doesn't have yet.
func (c *completer) keywords() {
	path := c.path
	if c.wantType && c.expected == nil {
		// A type: struct and interface first in a type declaration.
		structScore, otherScore := stdScore, highScore
		if len(path) > 1 {
			if _, ok := path[1].(*ast.TypeSpec); ok {
				structScore, otherScore = highScore, stdScore
			}
		}
		c.addKeywords(structScore, "struct", "interface")
		c.addKeywords(otherScore, "func", "chan", "map")
		return
	}
	if len(path) > 2 {
		// for i := r|: the range clause being typed (gopls).
		if loop, ok := path[2].(*ast.ForStmt); ok && loop.Init != nil && loop.Init.Pos() <= c.pos && c.pos <= loop.Init.End() {
			c.addKeywords(stdScore, "range")
		}
	}
	if !c.atStatement() {
		return
	}
	for i, n := range path {
		switch n := n.(type) {
		case *ast.CaseClause:
			if n.Colon.IsValid() && c.pos > n.Colon {
				c.addKeywords(stdScore, "break")
				if i+2 < len(path) {
					if _, ok := path[i+2].(*ast.SwitchStmt); ok {
						c.addKeywords(stdScore, "fallthrough")
					}
				}
			}
		case *ast.CommClause:
			if n.Colon.IsValid() && c.pos > n.Colon {
				c.addKeywords(stdScore, "break")
			}
		case *ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.SelectStmt:
			if !hasDefault(n) {
				c.addKeywords(highScore, "case", "default")
			}
		case *ast.ForStmt, *ast.RangeStmt:
			c.addKeywords(stdScore, "break", "continue")
		case *ast.FuncDecl, *ast.FuncLit:
			ret := "return"
			if sig := c.enclosingSignature(); sig != nil && needsResults(sig) {
				ret += " " // a value must follow (gopls's requireReturnObj)
			}
			c.addKeywords(stdScore, stmtKeywords...)
			c.replaceKeyword("return", ret)
			return
		}
	}
}

// needsResults reports whether a return in a function with the signature
// sig must give values: it has results, and none has a name but _.
func needsResults(sig *types.Signature) bool {
	if sig.Results().Len() == 0 {
		return false
	}
	for v := range sig.Results().Variables() {
		if v.Name() != "" && v.Name() != "_" {
			return false
		}
	}
	return true
}

// atStatement reports whether a statement can start at the cursor: the
// name being typed is a statement of its own, or there is no name and the
// cursor is between the statements of a block or a clause.
func (c *completer) atStatement() bool {
	path := c.path
	switch n := path[0].(type) {
	case *ast.Ident:
		if len(path) < 2 {
			return false
		}
		switch p := path[1].(type) {
		case *ast.BlockStmt:
			return true
		case *ast.ExprStmt:
			return p.X == n
		case *ast.CaseClause:
			return p.Colon.IsValid() && c.pos > p.Colon
		case *ast.CommClause:
			return p.Colon.IsValid() && c.pos > p.Colon
		}
	case *ast.BlockStmt:
		return c.start == c.end
	case *ast.CaseClause:
		return c.start == c.end && n.Colon.IsValid() && c.pos > n.Colon
	case *ast.CommClause:
		return c.start == c.end && n.Colon.IsValid() && c.pos > n.Colon
	}
	return false
}

// hasDefault reports whether the switch or select statement n has a
// default clause.
func hasDefault(n ast.Node) bool {
	var body *ast.BlockStmt
	switch n := n.(type) {
	case *ast.SwitchStmt:
		body = n.Body
	case *ast.TypeSwitchStmt:
		body = n.Body
	case *ast.SelectStmt:
		body = n.Body
	}
	if body == nil {
		return false
	}
	for _, s := range body.List {
		switch cl := s.(type) {
		case *ast.CaseClause:
			if cl.List == nil {
				return true
			}
		case *ast.CommClause:
			if cl.Comm == nil {
				return true
			}
		}
	}
	return false
}

// addKeywords adds the keywords that fit the prefix, each once.
func (c *completer) addKeywords(score float64, kws ...string) {
	for _, kw := range kws {
		if !c.match(kw) || slices.ContainsFunc(c.cands, func(x *candidate) bool { return x.keyword && x.name == kw }) {
			continue
		}
		c.cands = append(c.cands, &candidate{name: kw, kind: protocol.KeywordCompletion, score: score, keyword: true})
	}
}

// replaceKeyword changes the text that a keyword candidate inserts
// (return → "return ").
func (c *completer) replaceKeyword(kw, text string) {
	for _, x := range c.cands {
		if x.keyword && x.name == kw && text != kw {
			x.insert = text
		}
	}
}
