package check

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"slices"
	"strconv"
	"strings"

	"github.com/huuhait/gocpp/internal/diag"
	"github.com/huuhait/gocpp/internal/load"
	"github.com/huuhait/gocpp/internal/syntax/gpast"
)

// The `!` (must) forms of Phase 10 (README §2.1, docs/adr/037-must-rewrite.md):
//
//	func loadConfig!(path string) Config   a must function
//	cfg := loadConfig!(path)               a must call
//
// The parser makes them stock go/ast nodes with marks (package gpast): a
// name that ends with "!". go/types does not see the marks (it reports
// nothing for F!(x), see gpast), so Check rewrites the package before it
// calls go/types, and again between its checks. The pass has four steps:
//
//  1. prepare (must.go): a must function gets its error result (unless the
//     last declared result is error), "return x" becomes "return x, nil",
//     and each must call F!(a) becomes the call of a hidden generic helper
//     gp_must.M1(F(a)) (mustpkg.go) that the stock go/types can infer the
//     types of, chosen by what the call is part of: a statement is
//     gp_must.Any(F(a)), a, b := F!(x) is gp_must.M2(F(x)), anything else
//     is M1. The file gets an import gp_must "gocpp/must".
//  2. rewriteDropped (mustdrop.go): inside a must function, a call that
//     drops its error (a statement, or fewer values than it returns:
//     data := os.ReadFile(path)) gets a helper too. Which calls drop their
//     errors depends on the types of the callees, so this needs a check
//     first; it runs again after each check until no call is left, because
//     a call on a value that the first check couldn't type
//     (v := os.Open(p); v.Close()) is only known after the next one.
//  3. postCheck (mustdrop.go): when go/types is done, every helper call is
//     checked: the last result of the call must be exactly error
//     (types.Identical: a *MyErr that is assignable to error would turn a
//     nil pointer into a non-nil error), and the number of values must be
//     the one the helper gives. Errors say what is wrong in the words of
//     the user's code, and the go/types errors about the helpers go.
//  4. lower (mustlower.go): each helper becomes the code that a programmer
//     writes by hand: v, must_e := F(a); if must_e != nil { return ..., must_e }.
//     No helper is left (except gp_must.Exit, the end of main!), so the
//     lowerer and the C++ never see one. One more check gives the types of
//     the new code.
//
// When there are errors (of the user, or of the rewrite), the lowering does
// not run, and the AST keeps its helpers (positions are the ones of the
// user's text: a helper call has the span of F!(a)).
type mustPass struct {
	fset  *token.FileSet
	lp    *load.Package
	files []*ast.File

	// name is the name of the import of gocpp/must in the files that have
	// helpers: gp_must, or gp_must_N if the package uses gp_must already.
	name string

	// imported holds the files that have the import.
	imported map[*ast.File]bool

	// wrappers are the real main and init functions of main! and init!, by the function
	// that they call (renamed in place), added after it when prepare is done.
	wrappers map[*ast.FuncDecl]*ast.FuncDecl

	// taken holds the names of all identifiers in the package, for fresh
	// names (must_t1, must_e1, ...), or is nil until the first fresh name.
	taken map[string]bool

	// funcs holds the must functions (declared with !), and order the same
	// in source order.
	funcs map[*ast.FuncDecl]*mustFunc
	order []*mustFunc

	// calls are the helper calls, in the order they were made.
	calls    []*mustCall
	byHelper map[*ast.CallExpr]*mustCall

	// errs are the errors of the rewrite itself: a ! that is misplaced, a
	// call that can't be handled, a call whose last result is not error.
	errs []diag.Diagnostic

	// rewrites counts the rewrites of rewriteDropped in all rounds, to stop
	// a loop that does not end.
	rounds int

	// info is what Package.Must says; noted holds the calls that are in it (mustinfo.go).
	info  MustInfo
	noted map[*mustCall]bool
}

// A mustFunc is a must function: func f!(...).
type mustFunc struct {
	decl *ast.FuncDecl
	file *ast.File

	// special is mustMain for func main!() of package main, mustInit for
	// func init!(): they get no error result, the error is printed and the
	// program exits with 1.
	special int

	// added says that prepare added the error result.
	added bool

	// name is the name the user wrote (main, init); exit says that the function is main! or init!,
	// whose error the real main or init prints before the program exits (Package.Must).
	name string
	exit bool
}

const (
	mustPlain = iota
	mustMain
	mustInit
)

// A mustCall is a helper call that the pass made: gp_must.Mk(inner).
type mustCall struct {
	helper *ast.CallExpr
	inner  *ast.CallExpr
	file   *ast.File

	// explicit says that the user wrote F!(...); otherwise the pass made
	// the helper because a must function drops the error of the call.
	explicit bool

	// deferred says that the call is the call of a defer statement.
	deferred bool

	// fn is the function that the call is in: a func declaration or a func
	// literal (nil at package level).
	fn ast.Node
}

// arity returns the number of values that the helper gives.
func (c *mustCall) arity() int {
	return mustArity(c.helper.Fun.(*ast.SelectorExpr).Sel.Name)
}

// newMustPass returns the pass for the package lp, or nil if no file has a
// form of Phase 10 (a must function, a must call, a macro: gpast.Leftovers).
func newMustPass(fset *token.FileSet, lp *load.Package) *mustPass {
	var mp *mustPass
	for _, f := range lp.Files {
		if f.AST == nil || len(gpast.Leftovers(f.AST)) == 0 {
			continue
		}
		if mp == nil {
			mp = &mustPass{
				fset:     fset,
				lp:       lp,
				imported: make(map[*ast.File]bool),
				wrappers: make(map[*ast.FuncDecl]*ast.FuncDecl),
				funcs:    make(map[*ast.FuncDecl]*mustFunc),
				byHelper: make(map[*ast.CallExpr]*mustCall),
			}
		}
	}
	if mp != nil {
		for _, f := range lp.Files {
			mp.files = append(mp.files, f.AST)
		}
	}
	return mp
}

// errorf adds an error of the rewrite at the node n.
func (mp *mustPass) errorf(n ast.Node, format string, args ...any) {
	mp.errs = append(mp.errs, diag.Diagnostic{
		Pos: mp.fset.Position(n.Pos()),
		End: mp.fset.Position(n.End()),
		Msg: fmt.Sprintf(format, args...),
	})
}

// fresh returns a name that no identifier of the package has, made of the
// prefix and a number: must_t3.
func (mp *mustPass) fresh(prefix string, n *int) string {
	if mp.taken == nil {
		mp.taken = make(map[string]bool)
		for _, f := range mp.files {
			ast.Inspect(f, func(c ast.Node) bool {
				if id, ok := c.(*ast.Ident); ok {
					mp.taken[id.Name] = true
				}
				return true
			})
		}
	}
	for {
		*n++
		name := prefix + strconv.Itoa(*n)
		if !mp.taken[name] {
			mp.taken[name] = true
			return name
		}
	}
}

// freshName returns a name like fresh, with no number if the plain prefix is
// free: must_err, must_main.
func (mp *mustPass) freshName(prefix string) string {
	mp.fresh("", new(int)) // makes taken
	if !mp.taken[prefix] {
		mp.taken[prefix] = true
		return prefix
	}
	n := 0
	return mp.fresh(prefix+"_", &n)
}

// prepare is the first step: the signatures and returns of the must
// functions, and the helper calls of the must calls.
func (mp *mustPass) prepare() {
	for _, f := range mp.files {
		for _, d := range f.Decls {
			if fd, ok := d.(*ast.FuncDecl); ok && gpast.IsMust(fd.Name.Name) {
				mp.prepareFunc(f, fd)
			}
		}
	}
	for _, f := range mp.files {
		mp.scanFile(f, mp.visitExplicit)
	}
	// The real main or init follows the function it calls, so the order of the init functions
	// of a file stays the order of the source.
	if len(mp.wrappers) > 0 {
		for _, f := range mp.files {
			var decls []ast.Decl
			for _, d := range f.Decls {
				decls = append(decls, d)
				if fd, ok := d.(*ast.FuncDecl); ok && mp.wrappers[fd] != nil {
					decls = append(decls, mp.wrappers[fd])
				}
			}
			f.Decls = decls
		}
	}
	mp.reportLeftovers()
}

// prepareFunc rewrites the signature and the returns of the must function fd.
func (mp *mustPass) prepareFunc(f *ast.File, fd *ast.FuncDecl) {
	fd.Name.Name = gpast.MustBase(fd.Name.Name)
	mf := &mustFunc{decl: fd, file: f, name: fd.Name.Name}
	mp.funcs[fd] = mf
	mp.order = append(mp.order, mf)
	if fd.Recv == nil && fd.Name.Name == "init" || fd.Recv == nil && fd.Name.Name == "main" && f.Name.Name == "main" {
		if fd.Type.Params.NumFields() > 0 || fd.Type.Results.NumFields() > 0 {
			mp.errorf(fd.Name, "func %s! must have no parameters and no results (an error is printed and the program exits with 1)", fd.Name.Name)
			return
		}
		// func main!() is a function that returns error, called by the real main:
		//
		//	func must_main() error { ... }        (return err works in it)
		//	func main() { gp_must.Exit(must_main()) }
		//
		// Exit prints "<program>: <error>" to stderr and exits with 1 when the error is not nil.
		// main's defers run when must_main returns, before the error is printed.
		name := fd.Name.Name
		inner := "must_main"
		if name == "main" {
			inner = mp.freshName(inner)
		} else {
			n := 0
			inner = mp.fresh("must_init", &n)
		}
		wrapper := &ast.FuncDecl{
			Name: &ast.Ident{NamePos: fd.Name.NamePos, Name: name},
			Type: &ast.FuncType{Func: fd.Type.Func, Params: &ast.FieldList{}},
			Body: &ast.BlockStmt{Lbrace: fd.Body.Lbrace, Rbrace: fd.Body.Rbrace, List: []ast.Stmt{
				&ast.ExprStmt{X: mCall(
					&ast.SelectorExpr{X: mIdent(mp.importName(f)), Sel: mIdent(mustExit)},
					mCall(mIdent(inner)))},
			}},
		}
		fd.Name = &ast.Ident{NamePos: fd.Name.NamePos, Name: inner}
		mp.wrappers[fd] = wrapper
		mf.exit = true
	}
	if fd.Type.Results == nil {
		fd.Type.Results = &ast.FieldList{}
	}
	results := fd.Type.Results
	declared := results.NumFields()
	if declared > 0 {
		if id, ok := results.List[len(results.List)-1].Type.(*ast.Ident); ok && id.Name == "error" {
			return // the error is declared: the returns are normal Go
		}
	}
	named := declared > 0 && len(results.List[0].Names) > 0
	field := &ast.Field{Type: mIdent("error")}
	if named {
		n := 0
		field.Names = []*ast.Ident{mIdent(mp.fresh("must_err", &n))}
	}
	results.List = append(results.List, field)
	mf.added = true
	if fd.Body == nil {
		return
	}
	mustInspectOwn(fd.Body, func(n ast.Node) bool {
		if r, ok := n.(*ast.ReturnStmt); ok && len(r.Results) == declared && !(named && len(r.Results) == 0) {
			r.Results = append(r.Results, mIdent("nil")) // return x  becomes  return x, nil
		}
		return true
	})
	if declared == 0 && !mustTerminatesList(fd.Body.List) {
		fd.Body.List = append(fd.Body.List, &ast.ReturnStmt{Results: []ast.Expr{mIdent("nil")}})
	}
}

// calleeIdent returns the identifier that names the function that fun
// calls: F in F(x), pkg.F(x), x.F(a) and F[int](x).
func calleeIdent(fun ast.Expr) *ast.Ident {
	for {
		switch e := fun.(type) {
		case *ast.Ident:
			return e
		case *ast.SelectorExpr:
			return e.Sel
		case *ast.IndexExpr:
			fun = e.X
		case *ast.IndexListExpr:
			fun = e.X
		case *ast.ParenExpr:
			fun = e.X
		default:
			return nil
		}
	}
}

// mustLabel writes the callee fun of a must call the way it was written, with the ! after the name and before the
// type arguments: store.Run!, gen![int] (fun has lost its ! by now).
func mustLabel(fun ast.Expr) string {
	switch x := fun.(type) {
	case *ast.IndexExpr:
		return mustLabel(x.X) + "[" + types.ExprString(x.Index) + "]"
	case *ast.IndexListExpr:
		parts := make([]string, len(x.Indices))
		for i, ix := range x.Indices {
			parts[i] = types.ExprString(ix)
		}
		return mustLabel(x.X) + "[" + strings.Join(parts, ", ") + "]"
	}
	return types.ExprString(fun) + "!"
}

// visitExplicit is the visitor of prepare: it rewrites each call F!(...).
func (mp *mustPass) visitExplicit(st *site) {
	if st.helper != nil {
		return
	}
	id := calleeIdent(st.call.Fun)
	if id == nil || !gpast.IsMust(id.Name) {
		return
	}
	id.Name = gpast.MustBase(id.Name)
	arity := 1
	switch st.ctx {
	case ctxGo:
		mp.errorf(st.call, "go %s(...): the error of %s can't be returned to anyone from another goroutine; "+
			"call it in a func literal that handles the error", mustLabel(st.call.Fun), id.Name)
		return
	case ctxStmt, ctxDefer:
		arity = 0
	case ctxAssign:
		arity = st.n
	}
	if arity > 3 {
		mp.errorf(st.call, "%s!(...) can give at most 3 values besides the error, not %d", id.Name, arity)
		return
	}
	mp.wrap(st, arity, true)
}

// wrap replaces the call of the site by the helper call that gives arity
// values (0: Any), and adds the import of gocpp/must to its file.
func (mp *mustPass) wrap(st *site, arity int, explicit bool) *mustCall {
	inner := st.call
	pos := inner.Pos()
	name := mp.importName(st.file)
	helper := &ast.CallExpr{
		Fun: &ast.SelectorExpr{
			X:   &ast.Ident{NamePos: pos, Name: name},
			Sel: &ast.Ident{NamePos: pos, Name: mustHelper(arity)},
		},
		Lparen: inner.Lparen,
		Args:   []ast.Expr{inner},
		Rparen: inner.Rparen,
	}
	mc := &mustCall{helper: helper, inner: inner, file: st.file, explicit: explicit, deferred: st.ctx == ctxDefer}
	if st.fn != nil {
		mc.fn = st.fn.node()
	}
	mp.calls = append(mp.calls, mc)
	mp.byHelper[helper] = mc
	switch {
	case st.field != nil:
		*st.field = helper
	default:
		*st.slot = helper
	}
	return mc
}

// importName returns the name of the import of gocpp/must in f, and adds
// the import if f has none: at the import keyword of the first import
// declaration, or else in a new declaration at the package keyword (as
// converter.importName does for gocpp/cgoh).
func (mp *mustPass) importName(f *ast.File) string {
	if mp.name == "" {
		mp.name = "gp_must"
		used := make(map[string]bool)
		for _, f := range mp.files {
			ast.Inspect(f, func(c ast.Node) bool {
				if id, ok := c.(*ast.Ident); ok && strings.HasPrefix(id.Name, "gp_must") {
					used[id.Name] = true
				}
				return true
			})
		}
		for i := 1; used[mp.name]; i++ {
			mp.name = "gp_must_" + strconv.Itoa(i)
		}
	}
	if mp.imported[f] {
		return mp.name
	}
	newSpec := func(pos token.Pos) *ast.ImportSpec {
		return &ast.ImportSpec{
			Name: &ast.Ident{NamePos: pos, Name: mp.name},
			Path: &ast.BasicLit{ValuePos: pos, Kind: token.STRING, Value: strconv.Quote(load.MustPath)},
		}
	}
	var spec *ast.ImportSpec
	for _, d := range f.Decls {
		if gd, ok := d.(*ast.GenDecl); ok && gd.Tok == token.IMPORT {
			spec = newSpec(gd.TokPos)
			gd.Specs = append(gd.Specs, spec)
			break
		}
	}
	if spec == nil {
		spec = newSpec(f.Package)
		gd := &ast.GenDecl{TokPos: f.Package, Tok: token.IMPORT, Specs: []ast.Spec{spec}}
		f.Decls = slices.Insert(f.Decls, 0, ast.Decl(gd))
	}
	f.Imports = append(f.Imports, spec)
	mp.imported[f] = true
	return mp.name
}

// dropImports removes the import of gocpp/must from the files that don't
// use it any more (after the lowering), so that go/types doesn't report
// "imported and not used".
func (mp *mustPass) dropImports() {
	for _, f := range mp.files {
		if !mp.imported[f] {
			continue
		}
		used := false
		ast.Inspect(f, func(c ast.Node) bool {
			if id, ok := c.(*ast.Ident); ok && id.Name == mp.name {
				used = true
			}
			return !used
		})
		// The name of the import spec is an identifier too: look at the uses.
		uses := 0
		ast.Inspect(f, func(c ast.Node) bool {
			if sel, ok := c.(*ast.SelectorExpr); ok {
				if id, ok := sel.X.(*ast.Ident); ok && id.Name == mp.name {
					uses++
				}
			}
			return true
		})
		_ = used
		if uses > 0 {
			continue
		}
		delete(mp.imported, f)
		f.Imports = slices.DeleteFunc(f.Imports, func(s *ast.ImportSpec) bool { return s.Name != nil && s.Name.Name == mp.name })
		for i, d := range f.Decls {
			gd, ok := d.(*ast.GenDecl)
			if !ok || gd.Tok != token.IMPORT {
				continue
			}
			gd.Specs = slices.DeleteFunc(gd.Specs, func(s ast.Spec) bool {
				is := s.(*ast.ImportSpec)
				return is.Name != nil && is.Name.Name == mp.name
			})
			if len(gd.Specs) == 0 {
				f.Decls = slices.Delete(f.Decls, i, i+1)
			}
			break
		}
	}
}

// reportLeftovers reports the forms of Phase 10 that no rewrite handled
// (go/types would stay silent about most of them, see gpast): a ! that is
// not right before the arguments of a call, and the macros.
func (mp *mustPass) reportLeftovers() {
	for _, f := range mp.files {
		var callees map[*ast.Ident]bool
		for _, n := range gpast.Leftovers(f) {
			switch n := n.(type) {
			case *ast.Ident:
				if gpast.IsMust(n.Name) {
					if callees == nil {
						callees = calleeNames(f)
					}
					if callees[n] {
						// A call, but in a place no rewrite looks at: a constant, an array length, a type.
						mp.errorf(n, "%s(...): a must call can't be used in a constant, a type or an array length (the compiler computes them); use it in a statement or in the initializer of a variable", n.Name)
						continue
					}
					mp.errorf(n, "%s: a ! must come right before the arguments of a call, as in %s(...)", n.Name, n.Name)
					continue
				}
			}
			mp.errorf(n, "internal error: a macro form that the loader did not rewrite (internal/load/macros.go)")
		}
	}
}

// calleeNames returns the names that are called in f: f in f(x), the selector in p.f(x), and either with
// type arguments (f[int](x)).
func calleeNames(f *ast.File) map[*ast.Ident]bool {
	out := make(map[*ast.Ident]bool)
	ast.Inspect(f, func(n ast.Node) bool {
		c, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		fn := c.Fun
		for {
			switch x := fn.(type) {
			case *ast.IndexExpr:
				fn = x.X
				continue
			case *ast.IndexListExpr:
				fn = x.X
				continue
			}
			break
		}
		switch x := fn.(type) {
		case *ast.Ident:
			out[x] = true
		case *ast.SelectorExpr:
			out[x.Sel] = true
		}
		return true
	})
	return out
}

// A siteCtx says what a call is part of: it decides which values the call
// gives, and so how many results a helper gives.
type siteCtx int

const (
	ctxValue  siteCtx = iota // one value in an expression
	ctxStmt                  // an expression statement
	ctxDefer                 // the call of a defer statement
	ctxGo                    // the call of a go statement
	ctxAssign                // the only value of an assignment or a var declaration with n variables
	ctxArgs                  // the only argument of the call parent
	ctxReturn                // the only value of a return in a function with n results
)

// A site is a call in a function, with what it is part of.
type site struct {
	call *ast.CallExpr
	file *ast.File

	// slot holds the call, or field does (the call of a defer or go statement).
	slot  *ast.Expr
	field **ast.CallExpr

	ctx    siteCtx
	n      int
	parent *ast.CallExpr // for ctxArgs: the call that has call as its only argument

	// helper is the helper call of the pass if call is one (then the
	// helper's inner call is not a site of its own: its parts are).
	helper *mustCall

	fn *scanFn
}

// A scanFn is the function that a site is in.
type scanFn struct {
	decl *ast.FuncDecl
	lit  *ast.FuncLit
	typ  *ast.FuncType

	// must says that the function is a must function: it handles the errors
	// that its code drops.
	must *mustFunc
}

func (f *scanFn) node() ast.Node {
	if f.lit != nil {
		return f.lit
	}
	return f.decl
}

// results returns the number of results of the function.
func (f *scanFn) results() int {
	if f == nil || f.typ == nil {
		return 0
	}
	return f.typ.Results.NumFields()
}

// scanFile calls visit for each call in the file, in source order, with
// what the call is part of. visit may replace the call (st.slot or
// st.field) by a helper call; the scan goes on with the parts of the
// original call. It scans every function, including the func literals
// inside functions and in package-level variables.
func (mp *mustPass) scanFile(f *ast.File, visit func(*site)) {
	for _, d := range f.Decls {
		switch d := d.(type) {
		case *ast.FuncDecl:
			if d.Body != nil {
				mp.scanFunc(f, d.Body, &scanFn{decl: d, typ: d.Type, must: mp.funcs[d]}, visit)
			}
		case *ast.GenDecl:
			if d.Tok == token.VAR {
				for _, s := range d.Specs {
					mp.scanValueSpec(f, s.(*ast.ValueSpec), nil, visit)
				}
			}
		}
	}
}

func (mp *mustPass) scanFunc(f *ast.File, body *ast.BlockStmt, fn *scanFn, visit func(*site)) {
	for _, s := range body.List {
		mp.scanStmt(f, s, fn, visit)
	}
}

func (mp *mustPass) scanValueSpec(f *ast.File, vs *ast.ValueSpec, fn *scanFn, visit func(*site)) {
	if len(vs.Values) == 1 {
		if _, ok := vs.Values[0].(*ast.CallExpr); ok {
			mp.scanExpr(f, &vs.Values[0], ctxAssign, len(vs.Names), nil, fn, visit)
			return
		}
	}
	for i := range vs.Values {
		mp.scanExpr(f, &vs.Values[i], ctxValue, 1, nil, fn, visit)
	}
}

func (mp *mustPass) scanStmt(f *ast.File, s ast.Stmt, fn *scanFn, visit func(*site)) {
	value := func(slot *ast.Expr) {
		if *slot != nil {
			mp.scanExpr(f, slot, ctxValue, 1, nil, fn, visit)
		}
	}
	switch s := s.(type) {
	case nil:
	case *ast.ExprStmt:
		if _, ok := s.X.(*ast.CallExpr); ok {
			mp.scanExpr(f, &s.X, ctxStmt, 0, nil, fn, visit)
		} else {
			value(&s.X)
		}
	case *ast.AssignStmt:
		for i := range s.Lhs {
			value(&s.Lhs[i])
		}
		if len(s.Rhs) == 1 {
			if _, ok := s.Rhs[0].(*ast.CallExpr); ok {
				mp.scanExpr(f, &s.Rhs[0], ctxAssign, len(s.Lhs), nil, fn, visit)
				break
			}
		}
		for i := range s.Rhs {
			value(&s.Rhs[i])
		}
	case *ast.ReturnStmt:
		if len(s.Results) == 1 {
			if _, ok := s.Results[0].(*ast.CallExpr); ok {
				mp.scanExpr(f, &s.Results[0], ctxReturn, fn.results(), nil, fn, visit)
				break
			}
		}
		for i := range s.Results {
			value(&s.Results[i])
		}
	case *ast.DeferStmt:
		mp.scanDeferGo(f, s.Call, &s.Call, ctxDefer, fn, visit)
	case *ast.GoStmt:
		mp.scanDeferGo(f, s.Call, &s.Call, ctxGo, fn, visit)
	case *ast.DeclStmt:
		if gd, ok := s.Decl.(*ast.GenDecl); ok && gd.Tok == token.VAR {
			for _, sp := range gd.Specs {
				mp.scanValueSpec(f, sp.(*ast.ValueSpec), fn, visit)
			}
		}
	case *ast.BlockStmt:
		for _, c := range s.List {
			mp.scanStmt(f, c, fn, visit)
		}
	case *ast.IfStmt:
		mp.scanStmt(f, s.Init, fn, visit)
		value(&s.Cond)
		mp.scanStmt(f, s.Body, fn, visit)
		mp.scanStmt(f, s.Else, fn, visit)
	case *ast.ForStmt:
		mp.scanStmt(f, s.Init, fn, visit)
		value(&s.Cond)
		mp.scanStmt(f, s.Post, fn, visit)
		mp.scanStmt(f, s.Body, fn, visit)
	case *ast.RangeStmt:
		value(&s.Key)
		value(&s.Value)
		value(&s.X)
		mp.scanStmt(f, s.Body, fn, visit)
	case *ast.SwitchStmt:
		mp.scanStmt(f, s.Init, fn, visit)
		value(&s.Tag)
		for _, c := range s.Body.List {
			cc := c.(*ast.CaseClause)
			for i := range cc.List {
				value(&cc.List[i])
			}
			for _, b := range cc.Body {
				mp.scanStmt(f, b, fn, visit)
			}
		}
	case *ast.TypeSwitchStmt:
		mp.scanStmt(f, s.Init, fn, visit)
		mp.scanStmt(f, s.Assign, fn, visit)
		for _, c := range s.Body.List {
			for _, b := range c.(*ast.CaseClause).Body {
				mp.scanStmt(f, b, fn, visit)
			}
		}
	case *ast.SelectStmt:
		for _, c := range s.Body.List {
			cc := c.(*ast.CommClause)
			mp.scanStmt(f, cc.Comm, fn, visit)
			for _, b := range cc.Body {
				mp.scanStmt(f, b, fn, visit)
			}
		}
	case *ast.LabeledStmt:
		mp.scanStmt(f, s.Stmt, fn, visit)
	case *ast.IncDecStmt:
		value(&s.X)
	case *ast.SendStmt:
		value(&s.Chan)
		value(&s.Value)
	}
}

// scanDeferGo scans the call of a defer or go statement.
func (mp *mustPass) scanDeferGo(f *ast.File, c *ast.CallExpr, field **ast.CallExpr, ctx siteCtx, fn *scanFn, visit func(*site)) {
	st := &site{call: c, file: f, field: field, ctx: ctx, fn: fn}
	if h := mp.byHelper[c]; h != nil {
		st.helper = h
		c = h.inner
	}
	visit(st)
	mp.scanParts(f, c, fn, visit)
}

// scanParts scans what a call evaluates before it is called: the function
// and the arguments.
func (mp *mustPass) scanParts(f *ast.File, c *ast.CallExpr, fn *scanFn, visit func(*site)) {
	switch fun := c.Fun.(type) {
	case *ast.Ident:
	case *ast.SelectorExpr:
		mp.scanExpr(f, &fun.X, ctxValue, 1, nil, fn, visit)
	default:
		mp.scanExpr(f, &c.Fun, ctxValue, 1, nil, fn, visit)
	}
	if len(c.Args) == 1 && !c.Ellipsis.IsValid() {
		if _, ok := c.Args[0].(*ast.CallExpr); ok {
			mp.scanExpr(f, &c.Args[0], ctxArgs, 0, c, fn, visit)
			return
		}
	}
	for i := range c.Args {
		mp.scanExpr(f, &c.Args[i], ctxValue, 1, nil, fn, visit)
	}
}

func (mp *mustPass) scanExpr(f *ast.File, slot *ast.Expr, ctx siteCtx, n int, parent *ast.CallExpr, fn *scanFn, visit func(*site)) {
	switch e := (*slot).(type) {
	case *ast.FuncLit:
		lit := &scanFn{lit: e, typ: e.Type}
		mp.scanFunc(f, e.Body, lit, visit)
	case *ast.CallExpr:
		st := &site{call: e, file: f, slot: slot, ctx: ctx, n: n, parent: parent, fn: fn}
		inner := e
		if h := mp.byHelper[e]; h != nil {
			st.helper = h
			inner = h.inner
		}
		visit(st)
		mp.scanParts(f, inner, fn, visit)
	default:
		mustChildren(e, func(child ast.Node, cs *ast.Expr) {
			if cs != nil {
				mp.scanExpr(f, cs, ctxValue, 1, nil, fn, visit)
			}
		})
	}
}

// isTesting reports whether t is *testing.T, *testing.B, *testing.F or the interface testing.TB: the types
// whose Fatal a must call in a function that has no error result calls.
func isTesting(t types.Type) bool {
	if p, ok := t.(*types.Pointer); ok {
		n, ok := p.Elem().(*types.Named)
		if !ok || n.Obj().Pkg() == nil || n.Obj().Pkg().Path() != "testing" {
			return false
		}
		switch n.Obj().Name() {
		case "T", "B", "F":
			return true
		}
		return false
	}
	n, ok := t.(*types.Named)
	return ok && n.Obj().Pkg() != nil && n.Obj().Pkg().Path() == "testing" && n.Obj().Name() == "TB"
}

// identsOf returns the names of the syntax trees of lp.
func identsOf(lp *load.Package) map[*ast.Ident]bool {
	set := make(map[*ast.Ident]bool)
	for _, f := range lp.Files {
		if f.AST == nil {
			continue
		}
		ast.Inspect(f.AST, func(n ast.Node) bool {
			if id, ok := n.(*ast.Ident); ok {
				set[id] = true
			}
			return true
		})
	}
	return set
}

// markSynthetic records in each file of lp the names that are not in user (the names of the tree
// before the rewrite of the ! forms): File.Synthetic.
func markSynthetic(lp *load.Package, user map[*ast.Ident]bool) {
	for _, f := range lp.Files {
		if f.AST == nil {
			continue
		}
		// The names that the loader's macro rewrite wrote (internal/load/macros.go) are in user
		// (they were in the tree before this pass) and stay synthetic.
		ast.Inspect(f.AST, func(n ast.Node) bool {
			if id, ok := n.(*ast.Ident); ok && !user[id] {
				if f.Synthetic == nil {
					f.Synthetic = make(map[*ast.Ident]bool)
				}
				f.Synthetic[id] = true
			}
			return true
		})
	}
}
