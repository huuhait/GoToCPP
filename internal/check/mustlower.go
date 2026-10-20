package check

import (
	"go/ast"
	"go/token"
	"go/types"
	"reflect"
	"strconv"
)

// The lowering of the ! forms (step 10.5): the last step of the pass (see
// mustPass in must.go). When the checks are clean, each helper call
// gp_must.M1(F(a)) becomes the code that a programmer writes by hand, in the
// Go AST, and one more check gives the types of the new code. The lowerer
// of C++ then sees plain Go, so every context of the language that it
// supports works with ! at once, and no helper is left in the C++ (except
// gp_must.Exit, the end of the wrapper of main!, see lowerMain).
//
// What happens when the error is not nil depends on the function that the
// call is in (README §2.1):
//
//	failReturn  a function whose last result is error (every must function,
//	            main! too): return zero values and the error. Defers run.
//	failFatal   a function with a *testing.T, *testing.B or *testing.F
//	            parameter and no error result: t.Fatal(err).
//	failPanic   any other function (init and the initializers of package
//	            variables too): panic(err), Go's Must convention.
//
// The shapes, per context (the README's list; every one has a run test with
// a hand-written Go twin):
//
//   - a statement, or the value of an assignment: the call goes before the
//     statement:  v, must_e := F(a); if must_e != nil { return ..., must_e }; x := v
//     (a := F!(a) with plain names on the left needs no temporary);
//   - inside an expression: every call to the left of F!(a) is first moved
//     into a temporary, in Go's order, then F(a), so h() in g(h(), F!(a))
//     still runs before F;
//   - the right side of && and ||: the whole operation becomes a temporary
//     with an if around the right side's code, so short-circuiting is kept;
//   - if/else if: the code of the condition goes into a block around the
//     if (the else branch is a block when its condition has code);
//   - for: with ! in the condition, the loop becomes
//     for init; ; post { code; if !cond { break }; body }; with ! in the post
//     statement, a flag skips it on the first pass: continue still runs it;
//   - switch: with ! in a case, the switch becomes an if chain that finds
//     the case lazily, in source order, and a switch on its number;
//   - defer F!(a) and a deferred call that drops an error: the function
//     value, receiver and arguments are evaluated at the defer statement,
//     and a deferred closure calls F and handles the error at the exit.
type failKind int

const (
	failReturn failKind = iota
	failFatal
	failPanic
)

// A fnCtx is the function that the lowering is in: a func declaration or a
// func literal.
type fnCtx struct {
	kind failKind
	sig  *types.Signature
	typ  *ast.FuncType

	// node is the *ast.FuncDecl or *ast.FuncLit (nil for a package variable's initializer), and
	// exit says that the error ends the program (main!, init!, a main with F!: MustExit).
	node ast.Node
	exit bool

	// fatal is the name of the *testing.T parameter for failFatal.
	fatal string

	// errName is the name of the error result, once the results have names
	// (nameResults); errAlias, if not empty, is a pointer to it.
	errName  string
	errAlias string

	// gotos are the labels that a goto statement of the function (not of its
	// closures) jumps to; renamed maps such a label to the label that the
	// lowering put on the code before the labeled statement, so that a goto
	// runs that code again.
	gotos    map[string]bool
	branches map[string]bool // labels of break and continue statements
	renamed  map[string]string
}

type lowerer struct {
	mp   *mustPass
	info *types.Info
	pkg  *types.Package
	file *ast.File
	fn   *fnCtx

	tmp, errs, flags, labels int

	// later is set while the lowering is inside an operand that comes before
	// another operand with a ! call: everything in it that runs code must
	// run before that call, so what is left of it is moved into temporaries.
	later bool
}

// lower is the last step of the pass: it replaces every helper call. It
// returns false when it found a form that it can't lower (an error is
// reported then). The caller checks the package again.
func (mp *mustPass) lower(info *types.Info, pkg *types.Package) bool {
	l := &lowerer{mp: mp, info: info, pkg: pkg}
	nerr := len(mp.errs)
	for _, f := range mp.files {
		l.file = f
		var out []ast.Decl
		for _, d := range f.Decls {
			out = append(out, d)
			switch d := d.(type) {
			case *ast.FuncDecl:
				if w := l.lowerFuncDecl(d); w != nil {
					out = append(out, w)
				}
			case *ast.GenDecl:
				if d.Tok == token.VAR {
					l.fn = &fnCtx{kind: failPanic}
					for _, s := range d.Specs {
						l.lowerGlobalSpec(s.(*ast.ValueSpec))
					}
					l.lowerLits(d)
				}
			}
		}
		f.Decls = out
	}
	mp.dropImports()
	return len(mp.errs) == nerr
}

// has reports whether the tree n has a helper call that is not inside a
// func literal.
func (l *lowerer) has(n ast.Node) bool {
	if n == nil || reflect.ValueOf(n).IsNil() {
		return false
	}
	found := false
	ast.Inspect(n, func(c ast.Node) bool {
		if found {
			return false
		}
		switch c := c.(type) {
		case *ast.FuncLit:
			return false
		case *ast.CallExpr:
			if l.mp.byHelper[c] != nil {
				found = true
				return false
			}
		}
		return true
	})
	return found
}

func (l *lowerer) hasAny(ns ...ast.Node) bool {
	for _, n := range ns {
		if l.has(n) {
			return true
		}
	}
	return false
}

// at sets the position of the identifier id and returns it.
func (l *lowerer) at(id *ast.Ident, p token.Pos) *ast.Ident {
	id.NamePos = p
	return id
}

// freshT, freshE and freshF return new names for temporaries, error values
// and flags.
func (l *lowerer) freshT() string { return l.mp.fresh("must_t", &l.tmp) }
func (l *lowerer) freshE() string { return l.mp.fresh("must_e", &l.errs) }
func (l *lowerer) freshF() string { return l.mp.fresh("must_f", &l.flags) }

// --- functions ---

// sigOf returns the signature of a func literal or declaration.
func (l *lowerer) sigOfLit(lit *ast.FuncLit) *types.Signature {
	if tv, ok := l.info.Types[lit]; ok {
		if sig, ok := tv.Type.(*types.Signature); ok {
			return sig
		}
	}
	return nil
}

// newCtx makes the context of a function with the signature sig.
func (l *lowerer) newCtx(sig *types.Signature, typ *ast.FuncType, outerFatal string) *fnCtx {
	fc := &fnCtx{kind: failPanic, sig: sig, typ: typ}
	if sig != nil && sig.Results().Len() > 0 && mustIsError(sig.Results().At(sig.Results().Len()-1).Type()) {
		fc.kind = failReturn
		return fc
	}
	fc.fatal = outerFatal
	if typ != nil && typ.Params != nil && sig != nil {
		i := 0
		for _, field := range typ.Params.List {
			n := max(len(field.Names), 1)
			for k := 0; k < n; k++ {
				if i < sig.Params().Len() && isTesting(sig.Params().At(i).Type()) && k < len(field.Names) && field.Names[k].Name != "_" {
					fc.fatal = field.Names[k].Name
				}
				i++
			}
		}
	}
	if fc.fatal != "" {
		fc.kind = failFatal
	}
	return fc
}

// lowerFuncDecl lowers the body of a function. It returns the wrapper func
// that main! and init! get (see lowerMain), or nil.
func (l *lowerer) lowerFuncDecl(fd *ast.FuncDecl) *ast.FuncDecl {
	if fd.Body == nil {
		return nil
	}
	var wrapper *ast.FuncDecl
	mf := l.mp.funcs[fd]
	special := mf != nil && mf.special != mustPlain
	isMain := fd.Recv == nil && fd.Name.Name == "main" && l.file.Name.Name == "main"
	if !special && isMain && l.hasOwn(fd.Body) {
		special = true // a plain main with F!(...): the same end as main!
		if mf == nil {
			mf = &mustFunc{decl: fd, file: l.file, special: mustMain}
		}
	}
	var sig *types.Signature
	if obj, ok := l.info.Defs[fd.Name].(*types.Func); ok {
		sig = obj.Signature()
	}
	save := l.fn
	defer func() { l.fn = save }()
	if special {
		wrapper = l.lowerMain(fd, mf)
		l.fn = &fnCtx{kind: failReturn, typ: fd.Type, exit: true}
	} else {
		l.fn = l.newCtx(sig, fd.Type, "")
	}
	l.fn.node = fd
	l.lowerFunc(fd.Body)
	l.lowerLits(fd.Body)
	end := fd.Body.Rbrace
	l.anchorStmts(anchors{start: fd.Pos(), end: end, last: end}, fd.Body, fd.Type)
	if wrapper != nil {
		l.anchorStmts(anchors{start: fd.Pos(), end: end, last: end}, wrapper)
	}
	return wrapper
}

// hasOwn reports whether the body has a helper call that is not in a func
// literal.
func (l *lowerer) hasOwn(body *ast.BlockStmt) bool { return l.has(body) }

// lowerMain makes main! and init! (and a main with F!(...)) a function that
// returns an error, and returns the func that calls it: main (or init)
// becomes
//
//	func must_main() error { ... }
//	func main() { gp_must.Exit(must_main()) }
//
// where Exit prints "<program>: <error>" to stderr and exits with the code 1
// when the error is not nil (the lowerer emits gp::must_exit). main's
// defers run when must_main returns, before the error is printed.
func (l *lowerer) lowerMain(fd *ast.FuncDecl, mf *mustFunc) *ast.FuncDecl {
	name := fd.Name.Name
	base := "must_" + name
	var inner string
	if name == "main" {
		inner = l.mp.freshName(base)
	} else {
		n := 0
		inner = l.mp.fresh(base, &n)
	}
	wrapper := &ast.FuncDecl{
		Name: &ast.Ident{NamePos: fd.Name.NamePos, Name: name},
		Type: &ast.FuncType{Func: fd.Type.Func, Params: &ast.FieldList{}},
		Body: &ast.BlockStmt{List: []ast.Stmt{
			&ast.ExprStmt{X: mCall(
				&ast.SelectorExpr{X: mIdent(l.mp.importName(l.file)), Sel: mIdent(mustExit)},
				mCall(mIdent(inner)))},
		}},
	}
	fd.Name = &ast.Ident{NamePos: fd.Name.NamePos, Name: inner}
	fd.Type.Results = &ast.FieldList{List: []*ast.Field{{Type: mIdent("error")}}}
	mustInspectOwn(fd.Body, func(n ast.Node) bool {
		if r, ok := n.(*ast.ReturnStmt); ok && len(r.Results) == 0 {
			r.Results = []ast.Expr{mIdent("nil")}
		}
		return true
	})
	if !mustTerminatesList(fd.Body.List) {
		fd.Body.List = append(fd.Body.List, &ast.ReturnStmt{Results: []ast.Expr{mIdent("nil")}})
	}
	return wrapper
}

// lowerFunc lowers the statements of a function body (not the func
// literals inside it: lowerLits does them).
func (l *lowerer) lowerFunc(body *ast.BlockStmt) {
	var alias ast.Stmt
	if l.fn.kind == failReturn && l.hasDefer(body) {
		l.nameResults(l.fn)
		if l.shadowsResult(body, l.fn.errName) {
			// A deferred call sets the error result where the defer statement is, but the name
			// means another variable there: set it through a pointer taken at the start.
			l.fn.errAlias = l.mp.freshName("must_ep")
			alias = mDefine(mIdents(l.fn.errAlias), &ast.UnaryExpr{Op: token.AND, X: mIdent(l.fn.errName)})
		}
	}
	l.fn.gotos = make(map[string]bool)
	l.fn.branches = make(map[string]bool)
	l.fn.renamed = make(map[string]string)
	mustInspectOwn(body, func(n ast.Node) bool {
		if b, ok := n.(*ast.BranchStmt); ok && b.Label != nil {
			if b.Tok == token.GOTO {
				l.fn.gotos[b.Label.Name] = true
			} else {
				l.fn.branches[b.Label.Name] = true
			}
		}
		return true
	})
	body.List = l.lowerList(body.List)
	if alias != nil {
		l.anchorStmts(anchors{start: body.Lbrace, end: body.Lbrace + 1, last: body.Lbrace}, alias)
		body.List = append([]ast.Stmt{alias}, body.List...)
	}
	if len(l.fn.renamed) > 0 {
		mustInspectOwn(body, func(n ast.Node) bool {
			if b, ok := n.(*ast.BranchStmt); ok && b.Tok == token.GOTO && b.Label != nil {
				if m, ok := l.fn.renamed[b.Label.Name]; ok {
					b.Label = &ast.Ident{NamePos: b.Label.NamePos, Name: m}
				}
			}
			return true
		})
	}
}

// shadowsResult reports whether a variable of the function body (not of a closure in it) has the
// name of the error result: a name that means another variable inside its scope.
func (l *lowerer) shadowsResult(body *ast.BlockStmt, name string) bool {
	found := false
	mustInspectOwn(body, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok && id.Name == name {
			if _, ok := l.info.Defs[id].(*types.Var); ok {
				found = true
			}
		}
		return !found
	})
	return found
}

// lowerLits lowers the func literals in the tree n, each with its own
// signature, and the ones inside them.
func (l *lowerer) lowerLits(n ast.Node) {
	outer := l.fn
	ast.Inspect(n, func(c ast.Node) bool {
		lit, ok := c.(*ast.FuncLit)
		if !ok {
			return true
		}
		save := l.fn
		l.fn = l.newCtx(l.sigOfLit(lit), lit.Type, outer.fatal)
		l.fn.node = lit
		l.lowerFunc(lit.Body)
		l.lowerLits(lit.Body)
		l.fn = save
		return false
	})
}

// hasDefer reports whether the function body has a defer statement whose
// call is a helper (a deferred call that handles its error).
func (l *lowerer) hasDefer(body *ast.BlockStmt) bool {
	found := false
	mustInspectOwn(body, func(n ast.Node) bool {
		if d, ok := n.(*ast.DeferStmt); ok && l.mp.byHelper[d.Call] != nil {
			found = true
		}
		return !found
	})
	return found
}

// nameResults gives names to the results of the function, if it has none,
// and returns the name of the error result: a deferred call that fails sets
// it. If a variable of the function's code has the same name (it hides the
// result), the deferred code uses a pointer to the result.
func (l *lowerer) nameResults(fc *fnCtx) {
	if fc.errName != "" {
		return
	}
	res := fc.typ.Results
	named := len(res.List[0].Names) > 0
	for _, field := range res.List {
		if !named {
			field.Names = []*ast.Ident{l.at(mIdent(l.mp.freshName("must_r")), fc.typ.Func)}
		} else {
			for i, id := range field.Names {
				if id.Name == "_" {
					field.Names[i] = l.at(mIdent(l.mp.freshName("must_r")), fc.typ.Func)
				}
			}
		}
	}
	last := res.List[len(res.List)-1]
	fc.errName = last.Names[len(last.Names)-1].Name
}

// resultZeros returns fresh zero values of the results of the function
// before its last one.
func (l *lowerer) resultZeros() []ast.Expr {
	fc := l.fn
	if fc.sig == nil && fc.typ == nil {
		return nil
	}
	var zs []ast.Expr
	if fc.typ == nil || fc.typ.Results == nil {
		return nil
	}
	i := 0
	n := fc.typ.Results.NumFields()
	for _, field := range fc.typ.Results.List {
		k := max(len(field.Names), 1)
		for ; k > 0; k-- {
			if i < n-1 {
				var t types.Type
				if fc.sig != nil && i < fc.sig.Results().Len() {
					t = fc.sig.Results().At(i).Type()
				}
				zs = append(zs, l.zero(t, field.Type))
			}
			i++
		}
	}
	return zs
}

// zero returns the zero value of a type that is written as te in the
// source.
func (l *lowerer) zero(t types.Type, te ast.Expr) ast.Expr {
	if t == nil {
		return &ast.StarExpr{X: mCall(mIdent("new"), mCloneNode(te).(ast.Expr))}
	}
	if _, ok := types.Unalias(t).(*types.TypeParam); ok {
		return &ast.StarExpr{X: mCall(mIdent("new"), mCloneNode(te).(ast.Expr))}
	}
	switch u := t.Underlying().(type) {
	case *types.Basic:
		switch {
		case u.Info()&types.IsBoolean != 0:
			return mIdent("false")
		case u.Info()&types.IsString != 0:
			return &ast.BasicLit{Kind: token.STRING, Value: `""`}
		case u.Info()&types.IsNumeric != 0:
			return mInt("0")
		}
		return mIdent("nil")
	case *types.Pointer, *types.Slice, *types.Map, *types.Chan, *types.Signature, *types.Interface:
		return mIdent("nil")
	case *types.Struct, *types.Array:
		return &ast.CompositeLit{Type: mCloneNode(te).(ast.Expr)}
	}
	return &ast.StarExpr{X: mCall(mIdent("new"), mCloneNode(te).(ast.Expr))}
}

// failStmts returns the statements that run when the error e is not nil.
func (l *lowerer) failStmts(e string) []ast.Stmt {
	switch l.fn.kind {
	case failReturn:
		res := append(l.resultZeros(), mIdent(e))
		return []ast.Stmt{&ast.ReturnStmt{Results: res}}
	case failFatal:
		return []ast.Stmt{&ast.ExprStmt{X: mCall(&ast.SelectorExpr{X: mIdent(l.fn.fatal), Sel: mIdent("Fatal")}, mIdent(e))}}
	}
	return []ast.Stmt{&ast.ExprStmt{X: mCall(mIdent("panic"), mIdent(e))}}
}

func (l *lowerer) failIf(e string) ast.Stmt {
	return &ast.IfStmt{Cond: mNotNil(e), Body: mBlock(l.failStmts(e)...)}
}

// --- global variables ---

// lowerGlobalSpec lowers the initializers of a package variable: a
// function literal that is called at once, whose code panics on an error
// (there is no function to return from).
func (l *lowerer) lowerGlobalSpec(vs *ast.ValueSpec) {
	if !l.hasAny(exprNodes(vs.Values)...) {
		return
	}
	a := anchors{start: vs.Pos(), end: vs.End(), last: vs.End() - 1}
	defer func() {
		for _, v := range vs.Values {
			l.anchorStmts(a, v)
		}
	}()
	if len(vs.Values) == 1 && len(vs.Names) > 1 {
		// var a, b = F!(x): one literal that gives both values.
		var pre []ast.Stmt
		var results []ast.Expr
		if h := l.mp.byHelper[asCall(vs.Values[0])]; h != nil {
			vals := l.lowerH(h, &pre, nil, token.DEFINE)
			results = vals
		} else {
			l.expr(&vs.Values[0], &pre)
			results = []ast.Expr{vs.Values[0]}
		}
		resultTypes := make([]ast.Expr, len(vs.Names))
		for i, id := range vs.Names {
			te, ok := l.globalType(vs, i, id)
			if !ok {
				return
			}
			resultTypes[i] = te
		}
		body := append(pre, &ast.ReturnStmt{Results: results})
		vs.Values[0] = l.iife(resultTypes, body)
		return
	}
	for i := range vs.Values {
		if !l.has(vs.Values[i]) {
			continue
		}
		var te ast.Expr
		if i < len(vs.Names) {
			var ok bool
			if te, ok = l.globalType(vs, i, vs.Names[i]); !ok {
				return
			}
		}
		var pre []ast.Stmt
		l.expr(&vs.Values[i], &pre)
		body := append(pre, &ast.ReturnStmt{Results: []ast.Expr{vs.Values[i]}})
		vs.Values[i] = l.iife([]ast.Expr{te}, body)
	}
}

func (l *lowerer) globalType(vs *ast.ValueSpec, i int, id *ast.Ident) (ast.Expr, bool) {
	if vs.Type != nil {
		return mCloneNode(vs.Type).(ast.Expr), true
	}
	obj := l.info.Defs[id]
	if obj == nil {
		l.mp.errorf(id, "internal error: no type for the variable %s", id.Name)
		return nil, false
	}
	te, ok := l.typeExpr(obj.Type())
	if !ok {
		l.mp.errorf(id, "the type of %s (%s) can't be written in this file for the ! in its initializer; "+
			"declare the type: var %s T = ...", id.Name, obj.Type(), id.Name)
	}
	return te, ok
}

// iife returns func() (T...) { body }().
func (l *lowerer) iife(results []ast.Expr, body []ast.Stmt) ast.Expr {
	ft := &ast.FuncType{Params: &ast.FieldList{}, Results: &ast.FieldList{}}
	for _, r := range results {
		ft.Results.List = append(ft.Results.List, &ast.Field{Type: r})
	}
	return mCall(&ast.FuncLit{Type: ft, Body: mBlock(body...)})
}

func exprNodes(es []ast.Expr) []ast.Node {
	ns := make([]ast.Node, len(es))
	for i, e := range es {
		ns[i] = e
	}
	return ns
}

func asCall(e ast.Expr) *ast.CallExpr {
	c, _ := e.(*ast.CallExpr)
	return c
}

// typeExpr writes the type t as Go source in the current file. It returns
// false for a type that it can't write (an unnamed struct or interface with
// members, a type of a package that the file doesn't import).
func (l *lowerer) typeExpr(t types.Type) (ast.Expr, bool) {
	switch t := types.Unalias(t).(type) {
	case *types.Basic:
		if t.Info()&types.IsUntyped != 0 {
			return nil, false
		}
		return mIdent(t.Name()), true
	case *types.Named:
		var base ast.Expr
		obj := t.Obj()
		switch {
		case obj.Pkg() == nil:
			base = mIdent(obj.Name())
		case obj.Pkg() == l.pkg:
			base = mIdent(obj.Name())
		default:
			name, ok := l.importedAs(obj.Pkg())
			if !ok {
				return nil, false
			}
			base = &ast.SelectorExpr{X: mIdent(name), Sel: mIdent(obj.Name())}
		}
		if targs := t.TypeArgs(); targs != nil && targs.Len() > 0 {
			ix := &ast.IndexListExpr{X: base}
			for a := range targs.Types() {
				ae, ok := l.typeExpr(a)
				if !ok {
					return nil, false
				}
				ix.Indices = append(ix.Indices, ae)
			}
			return ix, true
		}
		return base, true
	case *types.TypeParam:
		return mIdent(t.Obj().Name()), true
	case *types.Pointer:
		e, ok := l.typeExpr(t.Elem())
		return &ast.StarExpr{X: e}, ok
	case *types.Slice:
		e, ok := l.typeExpr(t.Elem())
		return &ast.ArrayType{Elt: e}, ok
	case *types.Array:
		e, ok := l.typeExpr(t.Elem())
		return &ast.ArrayType{Len: mInt(strconv.FormatInt(t.Len(), 10)), Elt: e}, ok
	case *types.Map:
		k, ok1 := l.typeExpr(t.Key())
		v, ok2 := l.typeExpr(t.Elem())
		return &ast.MapType{Key: k, Value: v}, ok1 && ok2
	case *types.Chan:
		e, ok := l.typeExpr(t.Elem())
		dir := ast.SEND | ast.RECV
		switch t.Dir() {
		case types.SendOnly:
			dir = ast.SEND
		case types.RecvOnly:
			dir = ast.RECV
		}
		return &ast.ChanType{Dir: dir, Value: e}, ok
	case *types.Interface:
		if t.NumMethods() == 0 && t.NumEmbeddeds() == 0 {
			return mIdent("any"), true
		}
	case *types.Signature:
		ft := &ast.FuncType{Params: &ast.FieldList{}}
		list := func(tup *types.Tuple, variadic bool) (*ast.FieldList, bool) {
			fl := &ast.FieldList{}
			for i := range tup.Len() {
				pt := tup.At(i).Type()
				var e ast.Expr
				var ok bool
				if variadic && i == tup.Len()-1 {
					s, isSlice := pt.(*types.Slice)
					if !isSlice {
						return nil, false
					}
					e, ok = l.typeExpr(s.Elem())
					e = &ast.Ellipsis{Elt: e}
				} else {
					e, ok = l.typeExpr(pt)
				}
				if !ok {
					return nil, false
				}
				fl.List = append(fl.List, &ast.Field{Type: e})
			}
			return fl, true
		}
		var ok bool
		if ft.Params, ok = list(t.Params(), t.Variadic()); !ok {
			return nil, false
		}
		if t.Results().Len() > 0 {
			if ft.Results, ok = list(t.Results(), false); !ok {
				return nil, false
			}
		}
		return ft, true
	}
	return nil, false
}

// importedAs returns the name under which the current file imports pkg.
func (l *lowerer) importedAs(pkg *types.Package) (string, bool) {
	for _, im := range l.file.Imports {
		path, err := strconv.Unquote(im.Path.Value)
		if err != nil || path != pkg.Path() {
			continue
		}
		if im.Name != nil {
			if im.Name.Name == "_" || im.Name.Name == "." {
				continue
			}
			return im.Name.Name, true
		}
		return pkg.Name(), true
	}
	return "", false
}

// --- statements ---

// lowerList lowers a list of statements.
func (l *lowerer) lowerList(list []ast.Stmt) []ast.Stmt {
	var out []ast.Stmt
	for _, s := range list {
		a := stmtAnchors(s)
		had := l.has(s) // a statement that is lowered in place (a for loop) has new code too
		pre, main, scoped := l.stmt(s)
		if had || len(pre) > 0 || main != s {
			for _, p := range pre {
				l.anchorStmts(a, p)
			}
			l.anchorStmts(a, main)
		}
		// The temporaries of the code before a statement that declares no name of the user go
		// into a block of their own: a goto that jumps over the statement (to a label after it) is
		// legal Go, and must not jump over the declarations of the lowering.
		declares := false
		switch st := s.(type) {
		case *ast.AssignStmt:
			declares = st.Tok == token.DEFINE
		case *ast.DeclStmt, *ast.LabeledStmt:
			declares = true
		}
		wrapped := wrapScoped(pre, main, scoped || !declares)
		if len(pre) > 0 && len(wrapped) == 1 {
			l.anchorStmts(a, wrapped[0]) // a block that wrapScoped made
		}
		if had {
			l.mp.info.Stmts = append(l.mp.info.Stmts, MustStmt{Pos: a.start, End: a.end, Lowered: wrapped})
		}
		out = append(out, wrapped...)
	}
	return out
}

// stmtAnchors returns the anchors of the code made for the statement s.
func stmtAnchors(s ast.Stmt) anchors {
	a := anchors{start: s.Pos(), end: s.End()}
	a.last = a.end - 1
	if a.last < a.start {
		a.last = a.start
	}
	return a
}

// wrapScoped returns the statements for the code pre and the statement
// main: a block when the code must not leak its names (scoped).
func wrapScoped(pre []ast.Stmt, main ast.Stmt, scoped bool) []ast.Stmt {
	if scoped && len(pre) > 0 {
		list := pre
		if main != nil {
			list = append(slicesClip(pre), main)
		}
		return []ast.Stmt{mBlock(list...)}
	}
	if main != nil {
		return append(slicesClip(pre), main)
	}
	return pre
}

func slicesClip(s []ast.Stmt) []ast.Stmt { return s[:len(s):len(s)] }

// stmt lowers one statement. It returns the code that goes before it, the
// statement itself (nil if nothing is left of it), and whether the code
// and the statement must be in a block of their own (they declare names
// that belong to the statement: the init statement of an if, for or
// switch).
func (l *lowerer) stmt(s ast.Stmt) (pre []ast.Stmt, main ast.Stmt, scoped bool) {
	if s == nil {
		return nil, nil, false
	}
	if !l.has(s) {
		return nil, s, false
	}
	switch s := s.(type) {
	case *ast.ExprStmt:
		if h := l.mp.byHelper[asCall(s.X)]; h != nil {
			l.lowerH(h, &pre, nil, token.DEFINE)
			return pre, nil, false
		}
		l.expr(&s.X, &pre)
		return pre, s, false
	case *ast.AssignStmt:
		return l.assign(s)
	case *ast.ReturnStmt:
		l.seq(mSlots(s.Results), &pre)
		return pre, s, false
	case *ast.DeclStmt:
		return l.declStmt(s)
	case *ast.DeferStmt:
		return l.deferStmt(s)
	case *ast.GoStmt:
		l.callParts(s.Call, &pre)
		return pre, s, false
	case *ast.IncDecStmt:
		l.seq([]*ast.Expr{&s.X}, &pre)
		return pre, s, false
	case *ast.SendStmt:
		l.seq([]*ast.Expr{&s.Chan, &s.Value}, &pre)
		return pre, s, false
	case *ast.BlockStmt:
		s.List = l.lowerList(s.List)
		return nil, s, false
	case *ast.LabeledStmt:
		return l.labeledStmt(s)
	case *ast.IfStmt:
		return l.ifStmt(s)
	case *ast.ForStmt:
		return l.forStmt(s)
	case *ast.RangeStmt:
		l.seq([]*ast.Expr{&s.X}, &pre)
		s.Body.List = l.lowerList(s.Body.List)
		return pre, s, false
	case *ast.SwitchStmt:
		return l.switchStmt(s)
	case *ast.TypeSwitchStmt:
		return l.typeSwitchStmt(s)
	case *ast.SelectStmt:
		for _, c := range s.Body.List {
			cc := c.(*ast.CommClause)
			if l.has(cc.Comm) {
				l.mp.errorf(cc.Comm, "! in the channel operation of a select case is not supported: evaluate it before the select")
				continue
			}
			cc.Body = l.lowerList(cc.Body)
		}
		return nil, s, false
	}
	l.mp.errorf(s, "internal error: ! in a %T is not lowered", s)
	return nil, s, false
}

// labeledStmt lowers L: stmt. The code that the lowering puts before the
// statement (the calls of its ! forms) belongs to it: goto L runs the code
// again, and break L and continue L stay on the statement. A statement that
// is only code (L: x := F!(a)) gives the label to its first statement. For
// a statement that stays (a loop), the label stays on it, and a goto to it
// is changed to jump to a second label, on the first statement of the code.
func (l *lowerer) labeledStmt(s *ast.LabeledStmt) (pre []ast.Stmt, main ast.Stmt, scoped bool) {
	pre, main, scoped = l.stmt(s.Stmt)
	if len(pre) == 0 {
		if main == nil {
			main = &ast.EmptyStmt{}
		}
		s.Stmt = main
		return nil, s, scoped
	}
	if main == nil {
		pre[0] = &ast.LabeledStmt{Label: s.Label, Colon: s.Colon, Stmt: pre[0]}
		return pre, nil, false
	}
	s.Stmt = main
	if !l.fn.gotos[s.Label.Name] {
		return pre, s, scoped
	}
	l.labels++
	m := &ast.Ident{NamePos: s.Label.NamePos, Name: l.mp.fresh("must_L", &l.labels)}
	l.fn.renamed[s.Label.Name] = m.Name
	var keep ast.Stmt = s
	if !l.fn.branches[s.Label.Name] {
		keep = main // only gotos used the label: the statement needs none
	}
	if scoped {
		return nil, &ast.LabeledStmt{Label: m, Colon: s.Colon, Stmt: mBlock(append(slicesClip(pre), keep)...)}, false
	}
	pre[0] = &ast.LabeledStmt{Label: m, Colon: s.Colon, Stmt: pre[0]}
	return pre, keep, false
}

func mSlots(es []ast.Expr) []*ast.Expr {
	slots := make([]*ast.Expr, len(es))
	for i := range es {
		slots[i] = &es[i]
	}
	return slots
}

// assign lowers an assignment or a short variable declaration.
func (l *lowerer) assign(s *ast.AssignStmt) (pre []ast.Stmt, main ast.Stmt, scoped bool) {
	if len(s.Rhs) == 1 && s.Tok == token.DEFINE {
		if h := l.mp.byHelper[asCall(s.Rhs[0])]; h != nil && h.arity() == len(s.Lhs) && allIdents(s.Lhs) {
			// a, b := F!(x): a, b, must_e := F(x), as a programmer writes it.
			l.lowerH(h, &pre, s.Lhs, token.DEFINE)
			return pre, nil, false
		}
	}
	var slots []*ast.Expr
	if s.Tok != token.DEFINE {
		slots = append(slots, mSlots(s.Lhs)...)
	}
	slots = append(slots, mSlots(s.Rhs)...)
	if len(s.Rhs) == 1 {
		if h := l.mp.byHelper[asCall(s.Rhs[0])]; h != nil && h.arity() > 1 {
			// x, y = F!(a): temporaries, then the assignment.
			if s.Tok != token.DEFINE {
				l.seq(mSlots(s.Lhs), &pre)
			}
			vals := l.lowerH(h, &pre, nil, token.DEFINE)
			s.Rhs = vals
			return pre, s, false
		}
	}
	l.seq(slots, &pre)
	return pre, s, false
}

func allIdents(es []ast.Expr) bool {
	for _, e := range es {
		if _, ok := e.(*ast.Ident); !ok {
			return false
		}
	}
	return true
}

// declStmt lowers a var declaration inside a function.
func (l *lowerer) declStmt(s *ast.DeclStmt) (pre []ast.Stmt, main ast.Stmt, scoped bool) {
	gd, ok := s.Decl.(*ast.GenDecl)
	if !ok || gd.Tok != token.VAR {
		return nil, s, false
	}
	// A declaration of several specs (var ( ... )) that has a ! call is split into one declaration per
	// spec, in order: the code of each spec goes right before it, a later spec sees the names of the
	// earlier ones, and the specs run in their source order.
	found := false
	for _, sp := range gd.Specs {
		if l.hasAny(exprNodes(sp.(*ast.ValueSpec).Values)...) {
			found = true
		}
	}
	if !found {
		return nil, s, false
	}
	emit := func(vs *ast.ValueSpec) {
		pre = append(pre, &ast.DeclStmt{Decl: &ast.GenDecl{Tok: token.VAR, Specs: []ast.Spec{vs}}})
	}
	for _, sp := range gd.Specs {
		vs := sp.(*ast.ValueSpec)
		if !l.hasAny(exprNodes(vs.Values)...) {
			emit(vs)
			continue
		}
		if len(vs.Values) == 1 && vs.Type == nil {
			if h := l.mp.byHelper[asCall(vs.Values[0])]; h != nil && h.arity() == len(vs.Names) {
				// var a, b = F!(x): var a, b, must_e = F(x), then the error check.
				l.mp.noteCall(h, l.fn)
				l.callParts(h.inner, &pre)
				e := l.freshE()
				vs.Names = append(vs.Names, mIdent(e))
				vs.Values = []ast.Expr{h.inner}
				emit(vs)
				pre = append(pre, l.failIf(e))
				continue
			}
		}
		if len(vs.Values) == 1 && len(vs.Names) > 1 {
			if h := l.mp.byHelper[asCall(vs.Values[0])]; h != nil {
				vs.Values = l.lowerH(h, &pre, nil, token.DEFINE)
				emit(vs)
				continue
			}
		}
		l.seq(mSlots(vs.Values), &pre)
		emit(vs)
	}
	return pre, nil, false
}

// ifStmt lowers an if statement.
func (l *lowerer) ifStmt(s *ast.IfStmt) (pre []ast.Stmt, main ast.Stmt, scoped bool) {
	s.Body.List = l.lowerList(s.Body.List)
	if s.Else != nil {
		ePre, eMain, eScoped := l.stmt(s.Else)
		if _, isIf := eMain.(*ast.IfStmt); isIf && len(ePre) == 0 && !eScoped {
			s.Else = eMain
		} else if b, isBlock := eMain.(*ast.BlockStmt); isBlock && len(ePre) == 0 {
			s.Else = b
		} else {
			s.Else = mBlock(wrapScoped(ePre, eMain, false)...)
		}
	}
	if !l.has(s.Cond) {
		if s.Init != nil && l.has(s.Init) {
			var iMain ast.Stmt
			pre, iMain, _ = l.stmt(s.Init)
			s.Init = iMain
		}
		return pre, s, len(pre) > 0 // the names of the init statement belong to the if
	}
	// A helper in the condition: its code needs the init statement's names,
	// so everything goes into a block.
	if s.Init != nil {
		iPre, iMain, _ := l.stmt(s.Init)
		pre = append(pre, iPre...)
		if iMain != nil {
			pre = append(pre, iMain)
		}
		s.Init = nil
	}
	l.expr(&s.Cond, &pre)
	return pre, s, true
}

// forStmt lowers a for statement.
func (l *lowerer) forStmt(s *ast.ForStmt) (pre []ast.Stmt, main ast.Stmt, scoped bool) {
	s.Body.List = l.lowerList(s.Body.List)
	if s.Init != nil && l.has(s.Init) {
		if as, ok := s.Init.(*ast.AssignStmt); ok && as.Tok == token.DEFINE {
			// for i := F!(a); ...: the variables stay the loop's own (a new
			// one for each iteration, Go 1.22): only the values are computed
			// before the loop, for i := must_t1; ...
			l.forInit(as, &pre)
		} else {
			var iMain ast.Stmt
			pre, iMain, _ = l.stmt(s.Init)
			s.Init = iMain
		}
		scoped = true
	}
	hasCond, hasPost := l.has(s.Cond), l.has(s.Post)
	if !hasCond && !hasPost {
		return pre, s, scoped
	}
	scoped = true
	var head []ast.Stmt
	if hasPost {
		// for init; ; { if first { first = false } else { post }; cond; body }
		flag := l.freshF()
		pre = append(pre, mDefine(mIdents(flag), mIdent("true")))
		pPre, pMain, _ := l.stmt(s.Post)
		postCode := wrapScoped(pPre, pMain, false)
		head = append(head, &ast.IfStmt{
			Cond: mIdent(flag),
			Body: mBlock(mAssign(mIdent(flag), mIdent("false"))),
			Else: mBlock(postCode...),
		})
		s.Post = nil
	}
	if s.Cond != nil {
		cond := s.Cond
		var cPre []ast.Stmt
		l.expr(&cond, &cPre)
		head = append(head, cPre...)
		head = append(head, &ast.IfStmt{Cond: mNot(cond), Body: mBlock(&ast.BranchStmt{Tok: token.BREAK})})
		s.Cond = nil
	}
	s.Body.List = append(head, s.Body.List...)
	return pre, s, scoped
}

// forInit lowers the init statement i, j := A!(), B!() of a for statement
// to the computation of the values in pre; the statement stays, with the
// values in temporaries.
func (l *lowerer) forInit(as *ast.AssignStmt, pre *[]ast.Stmt) {
	if len(as.Rhs) == 1 {
		if h := l.mp.byHelper[asCall(as.Rhs[0])]; h != nil && h.arity() > 1 {
			as.Rhs = l.lowerH(h, pre, nil, token.DEFINE)
			return
		}
	}
	l.seq(mSlots(as.Rhs), pre)
}

// switchStmt lowers an expression switch.
func (l *lowerer) switchStmt(s *ast.SwitchStmt) (pre []ast.Stmt, main ast.Stmt, scoped bool) {
	caseH := false
	for _, c := range s.Body.List {
		cc := c.(*ast.CaseClause)
		if l.hasAny(exprNodes(cc.List)...) {
			caseH = true
		}
	}
	for _, c := range s.Body.List {
		cc := c.(*ast.CaseClause)
		cc.Body = l.lowerList(cc.Body)
	}
	if !caseH {
		if s.Tag != nil && l.has(s.Tag) {
			if s.Init != nil {
				iPre, iMain, _ := l.stmt(s.Init)
				pre = append(pre, iPre...)
				if iMain != nil {
					pre = append(pre, iMain)
				}
				s.Init = nil
				scoped = true
			}
			l.expr(&s.Tag, &pre)
		} else if s.Init != nil && l.has(s.Init) {
			var iMain ast.Stmt
			pre, iMain, _ = l.stmt(s.Init)
			s.Init = iMain
			scoped = len(pre) > 0 // the names of the init statement belong to the switch
		}
		return pre, s, scoped
	}
	// A helper in a case: find the case with an if chain, then switch on its number.
	if s.Init != nil {
		iPre, iMain, _ := l.stmt(s.Init)
		pre = append(pre, iPre...)
		if iMain != nil {
			pre = append(pre, iMain)
		}
	}
	tag := ""
	if s.Tag != nil {
		tag = l.freshT()
		tPre := []ast.Stmt{}
		if l.has(s.Tag) {
			l.expr(&s.Tag, &tPre)
		}
		pre = append(pre, tPre...)
		pre = append(pre, mDefine(mIdents(tag), s.Tag))
	}
	idx := l.freshT()
	pre = append(pre, mDefine(mIdents(idx), &ast.UnaryExpr{Op: token.SUB, X: mInt("1")}))
	var build func(i int) []ast.Stmt
	clauses := s.Body.List
	build = func(i int) []ast.Stmt {
		for i < len(clauses) && clauses[i].(*ast.CaseClause).List == nil {
			i++ // default
		}
		if i >= len(clauses) {
			return nil
		}
		cc := clauses[i].(*ast.CaseClause)
		var cond ast.Expr
		for _, e := range cc.List {
			if tag != "" {
				e = &ast.BinaryExpr{X: mIdent(tag), Op: token.EQL, Y: e}
			}
			if cond == nil {
				cond = e
			} else {
				cond = &ast.BinaryExpr{X: cond, Op: token.LOR, Y: e}
			}
		}
		var cPre []ast.Stmt
		l.expr(&cond, &cPre)
		ifs := &ast.IfStmt{Cond: cond, Body: mBlock(mAssign(mIdent(idx), mInt(strconv.Itoa(i))))}
		if rest := build(i + 1); len(rest) > 0 {
			ifs.Else = mBlock(rest...)
		}
		return append(cPre, ifs)
	}
	pre = append(pre, build(0)...)
	sw := &ast.SwitchStmt{Switch: s.Switch, Tag: mIdent(idx), Body: s.Body}
	for i, c := range clauses {
		cc := c.(*ast.CaseClause)
		if cc.List != nil {
			cc.List = []ast.Expr{mInt(strconv.Itoa(i))}
		}
	}
	return pre, sw, true
}

// typeSwitchStmt lowers a type switch: only its init statement and the
// expression that it switches on can have a helper.
func (l *lowerer) typeSwitchStmt(s *ast.TypeSwitchStmt) (pre []ast.Stmt, main ast.Stmt, scoped bool) {
	for _, c := range s.Body.List {
		cc := c.(*ast.CaseClause)
		cc.Body = l.lowerList(cc.Body)
	}
	if s.Init != nil && (l.has(s.Init) || l.has(s.Assign)) {
		iPre, iMain, _ := l.stmt(s.Init)
		pre = append(pre, iPre...)
		if iMain != nil {
			pre = append(pre, iMain)
		}
		s.Init = nil
		scoped = true
	}
	if l.has(s.Assign) {
		var x *ast.Expr
		switch a := s.Assign.(type) {
		case *ast.ExprStmt:
			x = &a.X
		case *ast.AssignStmt:
			x = &a.Rhs[0]
		}
		if ta, ok := (*x).(*ast.TypeAssertExpr); ok {
			l.seq([]*ast.Expr{&ta.X}, &pre)
		}
		scoped = true
	}
	return pre, s, scoped
}

// deferStmt lowers a defer statement.
func (l *lowerer) deferStmt(s *ast.DeferStmt) (pre []ast.Stmt, main ast.Stmt, scoped bool) {
	h := l.mp.byHelper[s.Call]
	if h == nil {
		l.callParts(s.Call, &pre)
		return pre, s, false
	}
	inner := h.inner
	l.mp.noteCall(h, l.fn)
	l.callParts(inner, &pre)
	// The function value, the receiver and the arguments are evaluated now.
	call := &ast.CallExpr{Lparen: inner.Lparen, Rparen: inner.Rparen, Ellipsis: inner.Ellipsis}
	call.Fun = l.freezeFun(inner.Fun, &pre)
	tuple := false
	if len(inner.Args) == 1 && !inner.Ellipsis.IsValid() {
		if c := asCall(inner.Args[0]); c != nil {
			if res, ok := callResults(l.info, c); ok && len(res) > 1 {
				var names []ast.Expr
				for range res {
					names = append(names, mIdent(l.freshT()))
				}
				pre = append(pre, mDefine(names, inner.Args[0]))
				call.Args = names
				tuple = true
			}
		}
	}
	if !tuple {
		for _, a := range inner.Args {
			call.Args = append(call.Args, l.freezeArg(a, &pre))
		}
	}
	res, _ := callResults(l.info, inner)
	var lhs []ast.Expr
	for range max(len(res)-1, 0) {
		lhs = append(lhs, mIdent("_"))
	}
	e := l.freshE()
	lhs = append(lhs, mIdent(e))
	var body []ast.Stmt
	switch l.fn.kind {
	case failReturn:
		target := l.fn.errName
		var set ast.Expr = mIdent(target)
		if l.fn.errAlias != "" {
			set = &ast.StarExpr{X: mIdent(l.fn.errAlias)}
		}
		body = []ast.Stmt{&ast.IfStmt{
			Init: &ast.AssignStmt{Lhs: lhs, Tok: token.DEFINE, Rhs: []ast.Expr{call}},
			Cond: &ast.BinaryExpr{X: mNotNil(e), Op: token.LAND, Y: &ast.BinaryExpr{X: set, Op: token.EQL, Y: mIdent("nil")}},
			Body: mBlock(mAssign(set, mIdent(e))),
		}}
	default:
		// The function has no error to return. If a panic is running, the error of the
		// call is dropped and the panic goes on: recover it, make the call, panic again.
		r := l.freshT()
		body = []ast.Stmt{
			mDefine(mIdents(r), mCall(mIdent("recover"))),
			&ast.AssignStmt{Lhs: lhs, Tok: token.DEFINE, Rhs: []ast.Expr{call}},
			&ast.IfStmt{Cond: &ast.BinaryExpr{X: mIdent(r), Op: token.NEQ, Y: mIdent("nil")},
				Body: mBlock(&ast.ExprStmt{X: mCall(mIdent("panic"), mIdent(r))})},
			&ast.IfStmt{Cond: mNotNil(e), Body: mBlock(l.failStmts(e)...)},
		}
	}
	lit := &ast.FuncLit{Type: &ast.FuncType{Params: &ast.FieldList{}}, Body: mBlock(body...)}
	return pre, &ast.DeferStmt{Defer: s.Defer, Call: mCall(lit)}, false
}

// freezeFun returns an expression for the function of a deferred call that
// stands for the value it has now: a function declared at package level
// and a qualified function stay as they are; a method becomes a method
// value, which evaluates (and keeps) the receiver; anything else is copied
// into a variable.
func (l *lowerer) freezeFun(fun ast.Expr, pre *[]ast.Stmt) ast.Expr {
	switch f := ast.Unparen(fun).(type) {
	case *ast.Ident:
		if _, ok := l.info.Uses[f].(*types.Func); ok {
			return fun
		}
		if _, ok := l.info.Uses[f].(*types.Builtin); ok {
			return fun
		}
	case *ast.SelectorExpr:
		if _, ok := l.info.Selections[f]; !ok {
			return fun // pkg.F
		}
	case *ast.IndexExpr, *ast.IndexListExpr:
		// parse[int]: a generic function with its type arguments
		if id := calleeIdent(fun); id != nil {
			if _, isFn := l.info.Uses[id].(*types.Func); isFn {
				return fun
			}
		}
	}
	name := l.freshT()
	*pre = append(*pre, mDefine(mIdents(name), fun))
	return mIdent(name)
}

// freezeArg returns an expression for an argument of a deferred call as it
// is now: a constant or nil stays, anything else is copied into a variable.
func (l *lowerer) freezeArg(a ast.Expr, pre *[]ast.Stmt) ast.Expr {
	if tv, ok := l.info.Types[a]; ok && (tv.Value != nil || tv.IsNil() || tv.IsType()) {
		return a
	}
	if _, ok := ast.Unparen(a).(*ast.FuncLit); !ok {
		if id, ok := ast.Unparen(a).(*ast.Ident); ok {
			if _, isFn := l.info.Uses[id].(*types.Func); isFn {
				return a
			}
		}
	}
	name := l.freshT()
	*pre = append(*pre, mDefine(mIdents(name), a))
	return mIdent(name)
}

// --- expressions ---

// callParts lowers the parts of a call that are evaluated before it is
// called (the function's receiver and the arguments), without the call.
func (l *lowerer) callParts(c *ast.CallExpr, pre *[]ast.Stmt) {
	if !l.has(c) {
		return
	}
	var slots []*ast.Expr
	switch fun := c.Fun.(type) {
	case *ast.Ident:
	case *ast.SelectorExpr:
		slots = append(slots, &fun.X)
	case *ast.IndexExpr, *ast.IndexListExpr:
		// fns[f()](x) evaluates f() before the arguments; F[int](x) (an
		// instance of a generic function) has nothing to evaluate.
		if id := calleeIdent(fun); id == nil || l.info.Instances[id].TypeArgs == nil {
			slots = append(slots, &c.Fun)
		}
	default:
		slots = append(slots, &c.Fun)
	}
	if len(c.Args) == 1 && !c.Ellipsis.IsValid() {
		if h := l.mp.byHelper[asCall(c.Args[0])]; h != nil && h.arity() > 1 {
			l.seq(slots, pre)
			c.Args = l.lowerH(h, pre, nil, token.DEFINE)
			return
		}
	}
	for i := range c.Args {
		slots = append(slots, &c.Args[i])
	}
	l.seq(slots, pre)
}

// seq lowers the operands in slots, which a construct evaluates in this
// order: the code of each operand with a helper call goes to pre, and
// every call before the last helper call is first moved into a temporary,
// so that Go's order of calls holds.
func (l *lowerer) seq(slots []*ast.Expr, pre *[]ast.Stmt) {
	saved := l.later
	defer func() { l.later = saved }()
	last := -1
	for i, s := range slots {
		if *s != nil && l.has(*s) {
			last = i
		}
	}
	if saved {
		last = len(slots) - 1 // an operand after these has a ! call: all of them come before it
	}
	for i := 0; i <= last; i++ {
		s := slots[i]
		if *s == nil {
			continue
		}
		// What is left of an operand that comes before a ! call (in this
		// sequence or in an outer one) has to run first: spill moves it.
		before := saved || i < last
		l.later = before
		if l.has(*s) {
			l.expr(s, pre)
			if before {
				l.spill(s, pre)
			}
		} else {
			l.spill(s, pre)
		}
	}
}

// expr lowers the expression in slot, which has a helper call: its code
// goes to pre, and the slot gets what is left of the expression.
func (l *lowerer) expr(slot *ast.Expr, pre *[]ast.Stmt) {
	e := *slot
	if c, ok := e.(*ast.CallExpr); ok {
		if h := l.mp.byHelper[c]; h != nil {
			vals := l.lowerH(h, pre, nil, token.DEFINE)
			if len(vals) == 1 {
				*slot = vals[0]
			} else if len(vals) == 0 {
				*slot = mIdent("nil") // a statement form in a value place: the check reported it
			} else {
				l.mp.errorf(c, "internal error: a helper that gives %d values in an expression", len(vals))
			}
			return
		}
		l.callParts(c, pre)
		return
	}
	if b, ok := e.(*ast.BinaryExpr); ok && (b.Op == token.LAND || b.Op == token.LOR) && l.has(b.Y) {
		// x && y with code in y: t := x; if t { code; t = y }
		t := l.freshT()
		if l.has(b.X) {
			l.expr(&b.X, pre)
		}
		*pre = append(*pre, mDefine(mIdents(t), b.X))
		var inner []ast.Stmt
		l.expr(&b.Y, &inner)
		inner = append(inner, mAssign(mIdent(t), b.Y))
		cond := ast.Expr(mIdent(t))
		if b.Op == token.LOR {
			cond = mNot(cond)
		}
		*pre = append(*pre, &ast.IfStmt{Cond: cond, Body: mBlock(inner...)})
		*slot = mIdent(t)
		return
	}
	var slots []*ast.Expr
	l.childSlots(e, &slots)
	l.seq(slots, pre)
}

// childSlots collects the operands of an expression in evaluation order.
func (l *lowerer) childSlots(e ast.Expr, slots *[]*ast.Expr) {
	switch e := e.(type) {
	case *ast.CompositeLit:
		isStruct := false
		if t := l.info.TypeOf(e); t != nil {
			_, isStruct = t.Underlying().(*types.Struct)
			if p, ok := t.Underlying().(*types.Pointer); ok {
				_, isStruct = p.Elem().Underlying().(*types.Struct)
			}
		}
		for i := range e.Elts {
			if kv, ok := e.Elts[i].(*ast.KeyValueExpr); ok {
				if !isStruct {
					*slots = append(*slots, &kv.Key)
				}
				*slots = append(*slots, &kv.Value)
				continue
			}
			*slots = append(*slots, &e.Elts[i])
		}
		return
	case *ast.FuncLit, *ast.BasicLit, *ast.Ident:
		return
	}
	mustChildren(e, func(child ast.Node, slot *ast.Expr) {
		if slot != nil {
			*slots = append(*slots, slot)
		}
	})
}

// spill moves the calls in the expression of slot, which has no helper
// call but comes before one, into temporaries (in order), so that they run
// before the helper call.
func (l *lowerer) spill(slot *ast.Expr, pre *[]ast.Stmt) {
	e := *slot
	if tv, ok := l.info.Types[e]; ok && tv.Value != nil {
		return
	}
	switch x := e.(type) {
	case *ast.FuncLit, *ast.BasicLit, *ast.Ident:
		return
	case *ast.CallExpr:
		if l.isUnit(x) {
			t := l.freshT()
			*pre = append(*pre, mDefine(mIdents(t), e))
			*slot = mIdent(t)
			return
		}
	case *ast.UnaryExpr:
		if x.Op == token.ARROW {
			t := l.freshT()
			*pre = append(*pre, mDefine(mIdents(t), e))
			*slot = mIdent(t)
			return
		}
	case *ast.BinaryExpr:
		if (x.Op == token.LAND || x.Op == token.LOR) && l.hasEffect(x.Y) {
			t := l.freshT()
			*pre = append(*pre, mDefine(mIdents(t), e))
			*slot = mIdent(t)
			return
		}
	}
	if !l.hasEffect(e) {
		return
	}
	var slots []*ast.Expr
	if c, ok := e.(*ast.CallExpr); ok {
		// a conversion or a built-in function without effects: its operands
		slots = mSlots(c.Args)
	} else {
		l.childSlots(e, &slots)
	}
	for _, s := range slots {
		if *s != nil {
			l.spill(s, pre)
		}
	}
}

// isUnit reports whether the call c is a call that runs code (not a
// conversion or a built-in function that only reads).
func (l *lowerer) isUnit(c *ast.CallExpr) bool {
	tv, ok := l.info.Types[c.Fun]
	if !ok {
		return true
	}
	if tv.IsType() {
		return false
	}
	if tv.IsBuiltin() {
		id, _ := ast.Unparen(c.Fun).(*ast.Ident)
		if id == nil {
			if sel, ok := ast.Unparen(c.Fun).(*ast.SelectorExpr); ok {
				id = sel.Sel // unsafe.Slice and the like
			}
		}
		if id != nil {
			switch id.Name {
			case "len", "cap", "make", "new", "min", "max", "real", "imag", "complex",
				"Sizeof", "Alignof", "Offsetof", "Add", "Slice", "SliceData", "String", "StringData":
				return false
			}
		}
	}
	return true
}

// hasEffect reports whether the expression has a call that runs code or a
// receive (outside of func literals).
func (l *lowerer) hasEffect(e ast.Expr) bool {
	found := false
	ast.Inspect(e, func(n ast.Node) bool {
		if found {
			return false
		}
		switch n := n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.CallExpr:
			if l.isUnit(n) {
				found = true
			}
		case *ast.UnaryExpr:
			if n.Op == token.ARROW {
				found = true
			}
		}
		return !found
	})
	return found
}

// lowerH lowers the helper call h: the code that evaluates the parts of the
// call, then the call and the check of its error, go to pre. The values of
// the call are in temporaries that lowerH returns, or in the variables lhs
// (a short variable declaration: the idents are new or redeclared).
func (l *lowerer) lowerH(h *mustCall, pre *[]ast.Stmt, lhs []ast.Expr, tok token.Token) []ast.Expr {
	// The call itself comes after its operands, which stay in its statement.
	saved := l.later
	l.later = false
	l.callParts(h.inner, pre)
	l.later = saved
	l.mp.noteCall(h, l.fn)
	res, _ := callResults(l.info, h.inner)
	n := max(len(res)-1, 0)
	var left, vals []ast.Expr
	switch {
	case lhs != nil:
		left = append(left, lhs...)
	case h.arity() == 0:
		for range n {
			left = append(left, mIdent("_"))
		}
	default:
		for range n {
			t := l.freshT()
			left = append(left, mIdent(t))
			vals = append(vals, mIdent(t))
		}
	}
	e := l.freshE()
	left = append(left, mIdent(e))
	*pre = append(*pre, &ast.AssignStmt{Lhs: left, Tok: token.DEFINE, Rhs: []ast.Expr{h.inner}})
	*pre = append(*pre, l.failIf(e))
	return vals
}

// mCloneNode returns a deep copy of an AST node (positions are kept).
func mCloneNode(n ast.Node) ast.Node {
	return mCloneValue(reflect.ValueOf(n)).Interface().(ast.Node)
}

func mCloneValue(v reflect.Value) reflect.Value {
	switch v.Kind() {
	case reflect.Pointer:
		if v.IsNil() {
			return v
		}
		if isASTObject(v.Type()) || v.Type() == commentType {
			return reflect.Zero(v.Type())
		}
		c := reflect.New(v.Type().Elem())
		c.Elem().Set(mCloneValue(v.Elem()))
		return c
	case reflect.Interface:
		if v.IsNil() {
			return v
		}
		c := reflect.New(v.Type()).Elem()
		c.Set(mCloneValue(v.Elem()))
		return c
	case reflect.Struct:
		c := reflect.New(v.Type()).Elem()
		for i := range v.NumField() {
			if c.Field(i).CanSet() {
				c.Field(i).Set(mCloneValue(v.Field(i)))
			}
		}
		return c
	case reflect.Slice:
		if v.IsNil() {
			return v
		}
		c := reflect.MakeSlice(v.Type(), v.Len(), v.Len())
		for i := range v.Len() {
			c.Index(i).Set(mCloneValue(v.Index(i)))
		}
		return c
	}
	return v
}

// --- positions ---

// The code that the lowering writes has no position (the nodes are made
// with token.NoPos), but the lowerer of C++ and go/types look at positions:
// a lambda captures the variables whose declarations are outside the
// literal (position in or out of its range), and the C++ line of a
// statement is the line of its position (debugging, panic traces). So after
// the lowering of a statement of the user, anchor gives every node of the
// code that the lowering made a position that maps to the statement:
//
//   - the keywords and brackets (if, return, {, }) and the names that are
//     used get the end of the statement, so that a use is never before the
//     declaration of the name it uses (go/types hides a name from the
//     positions before the end of its declaring statement);
//   - the names that are declared (v, gp_e := ...) get the start of the
//     statement, and inside a func literal that the lowering made (a
//     deferred call), the last character of the statement, a position in
//     the range of the literal, which no other declaration has.
//
// Nodes that already have a position (the user's code) are not changed.
type anchors struct {
	start, end, last token.Pos
}

func (l *lowerer) anchorStmts(a anchors, nodes ...ast.Node) {
	for _, n := range nodes {
		if n != nil && !reflect.ValueOf(n).IsNil() {
			anchorNode(n, a, a.start)
		}
	}
}

var noAnchorFields = map[string]bool{"Ellipsis": true, "Arrow": true, "Opening": true, "Closing": true}

func anchorNode(root ast.Node, a anchors, declPos token.Pos) {
	decls := make(map[*ast.Ident]bool)
	ast.Inspect(root, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.AssignStmt:
			if n.Tok == token.DEFINE {
				for _, e := range n.Lhs {
					if id, ok := e.(*ast.Ident); ok {
						decls[id] = true
					}
				}
			}
		case *ast.ValueSpec:
			for _, id := range n.Names {
				decls[id] = true
			}
		case *ast.RangeStmt:
			if n.Tok == token.DEFINE {
				for _, e := range []ast.Expr{n.Key, n.Value} {
					if id, ok := e.(*ast.Ident); ok {
						decls[id] = true
					}
				}
			}
		case *ast.Field:
			for _, id := range n.Names {
				decls[id] = true
			}
		}
		return true
	})
	var walk func(n ast.Node, declPos token.Pos)
	walk = func(n ast.Node, declPos token.Pos) {
		ast.Inspect(n, func(c ast.Node) bool {
			switch c := c.(type) {
			case *ast.Ident:
				if c.NamePos == token.NoPos {
					if decls[c] {
						c.NamePos = declPos
					} else {
						c.NamePos = a.end
					}
				}
				return true
			case *ast.FuncLit:
				if c.Type.Func == token.NoPos {
					c.Type.Func, c.Body.Lbrace, c.Body.Rbrace = a.last, a.last, a.last
					setTokens(c, a.end)
					walk(c.Body, a.last)
					walk(c.Type, a.last)
					return false
				}
				return true
			case *ast.FuncType:
				return true
			}
			if b, ok := c.(*ast.BlockStmt); ok && b.Lbrace == token.NoPos && b.Rbrace == token.NoPos {
				// A block that the lowering made holds the code of the statement: it spans the
				// statement, so tools that look for the node at a place find the user's code in it.
				b.Lbrace, b.Rbrace = a.start, a.last
			}
			setTokens(c, a.end)
			return true
		})
	}
	walk(root, declPos)
}

// setTokens gives the token positions of n that are NoPos the position p.
func setTokens(n ast.Node, p token.Pos) {
	v := reflect.ValueOf(n)
	if v.Kind() != reflect.Pointer || v.IsNil() {
		return
	}
	v = v.Elem()
	if v.Kind() != reflect.Struct {
		return
	}
	switch n.(type) {
	case *ast.FieldList, *ast.FuncType, *ast.ChanType, *ast.File, *ast.CommentGroup, *ast.Comment:
		return
	}
	for i := range v.NumField() {
		f := v.Field(i)
		if f.Type() == reflect.TypeFor[token.Pos]() && f.CanSet() && f.Int() == 0 && !noAnchorFields[v.Type().Field(i).Name] {
			f.SetInt(int64(p))
		}
	}
}

// isASTObject reports whether t is *ast.Object, the unresolved-identifier link that a clone drops
// (the type is deprecated, so it is found by name).
func isASTObject(t reflect.Type) bool {
	return t.Kind() == reflect.Pointer && t.Elem().PkgPath() == "go/ast" && t.Elem().Name() == "Object"
}
