package lower

import (
	"go/ast"
	"go/token"
	"go/types"
	"strconv"
	"strings"

	"github.com/huuhait/gocpp/internal/cppir"
)

// Func values and closures (Phase 03 README §2.6, step 3.11; the runtime is gocpp/func.hpp):
//
//	Go                          C++
//	func(int) string            gp::func<gp::string(int64_t)> (results: void, T or a std::tuple;
//	                            a parameter larger than 128 KiB is a const A&, as in decl.go)
//	var f func(), f = nil       gp::func<void()> f{};  f = nullptr
//	f == nil, f != nil          f == nullptr, f != nullptr
//	f = g (g a func)            f = gp::func_of<&gpkg_main::g>();
//	func(x int) int { ... }     gp::func<int64_t(int64_t)>([n]<int = 1>(int64_t x) mutable -> int64_t {
//	                                ...the body, one line per statement...
//	                            })
//	func() { ... }()            [n]<int = 1>() mutable -> void { ... }()  (called at once; over
//	                            several lines: [n]<...>(...) { ... }.operator()(), callValue)
//	f(x)                        f(x) (f a local that isn't boxed), else f.gp_call(x)
//	defer func() { ... }()      gp_df.gp_push_call([n]<int = 1>() mutable -> void { ... });
//	defer f(x)                  gp_df.gp_push_call(f, x);  (f and x evaluated now)
//
// A func literal is a C++ lambda with an explicit capture list: each variable of the functions
// around it that it uses (also in the func literals inside it) is captured by copy, [n]. A
// variable of at most 128 bytes that is never changed after its initialization and whose address
// is never taken (box.go doesn't box it) is copied into the closure when the closure is made,
// which gives the same values as sharing it would: it can't change. Every other captured variable
// is boxed (boxCaptured when it is changed, boxShared when it is larger: Go shares those too, so
// 2,000 closures of a big array don't hold 2,000 copies of it), so the copy is of its box,
// gp::ptr<T>, and the closure and the function share the variable, as in Go. The lambda is mutable, so the copies are not const
// (nothing changes them: they are never assigned).
//
// Its body is lowered like a function's (decl.go: funcBody): its parameters and named results
// (boxed ones copied into their boxes), a single-exit shape of its own if it has defers, its
// own break/continue/goto, with the names of the function around it (funcNames is shared, so its
// temporaries never clash with the function's). The printer writes the body over several lines,
// each statement at its .gp line (cppir.Lambda), so a debugger steps through a closure; a literal
// that is all on one .gp line stays on its C++ line.
//
// The template parameter <int = N> is the number of the func literal in the function (or func
// literal) around it, in source order from 1, as Go numbers them for its stack traces:
// main.main.func1, main.main.func1.1 (a literal in func1), main.init.func2 (in a package
// variable's initializer). The runtime's trace (runtime/src/trace.cpp) reads it from the
// mangled name of the lambda's operator()<N>, which both compilers keep, where their own
// numbers of the lambdas differ (g++ numbers them per parameter list, clang++ per file).
//
// Calling a func value: f(x) where nothing can change or free f while the call runs (a local
// variable that isn't boxed, a part of one, a temporary), else f.gp_call(x), which keeps the
// closure alive during the call (f = nil inside f). The func value is read after the calls of
// its scope, like any variable (gc reads it there too: f(g()) where g changes the package
// variable f calls the new f); a call that gives the func, f()(g()), comes before the arguments
// (order.go puts the callee first in the call's scope).
//
// Variadic functions take their last parameter as a slice: f(1, 2) is f(gp::slice_of<int64_t>({1,
// 2})), f() passes a nil slice, and f(s...) passes s itself (the callee shares s's array, as in
// Go).
//
// A closure that captures the box of the variable that holds it (var f func(); f = func() { f()
// }) is a reference cycle, which reference counting never frees (D1, the plan's risks): such a
// closure lives until the program ends.

// isFunc reports whether t is a func type (through names and aliases).
func isFunc(t types.Type) bool {
	_, ok := under(t).(*types.Signature)
	return ok
}

// funcType returns gp::func<R(P...)>, the C++ type of a func value of the signature sig: the
// parameters as a function declaration has them (prepare), the results as its result type.
func (g *gen) funcType(sig *types.Signature, pos token.Pos) cppir.Type {
	params := make([]cppir.Type, sig.Params().Len())
	for i := range params {
		t := sig.Params().At(i).Type()
		params[i] = g.varType(sig.Params().At(i), pos) // also a C/C++ type (cvalues.go)
		if largeVar(t) {
			params[i] = &cppir.Ref{Elem: &cppir.Const{Elem: params[i]}}
		}
	}
	return &cppir.TypeName{Name: "gp::func", Args: []cppir.Node{&cppir.FuncType{Result: g.resultType(sig, pos), Params: params}}}
}

// hasFunc reports whether a value of type t holds a func value (in a field, an element, a key,
// or behind a pointer): fmt and print can't print those yet (Go prints a func's code address).
func hasFunc(t types.Type) bool {
	seen := make(map[types.Type]bool)
	var walk func(t types.Type) bool
	walk = func(t types.Type) bool {
		if seen[t] {
			return false
		}
		seen[t] = true
		switch u := types.Unalias(t).Underlying().(type) {
		case *types.Signature:
			return true
		case *types.Pointer:
			return walk(u.Elem())
		case *types.Slice:
			return walk(u.Elem())
		case *types.Array:
			return walk(u.Elem())
		case *types.Map:
			return walk(u.Key()) || walk(u.Elem())
		case *types.Struct:
			for i := range u.NumFields() {
				if walk(u.Field(i).Type()) {
					return true
				}
			}
		}
		return false
	}
	return walk(t)
}

// numberLits numbers the func literals in n, in source order, as Go does for its stack traces
// (see the top of the file): in counts, each func literal (and the key nil, for n itself) has the
// number of the last literal directly inside it. A function declaration has its own counts; the
// initializers of the package variables share one, in source order (main.init.funcN).
func (g *gen) numberLits(n ast.Node, counts map[*ast.FuncLit]int) {
	if g.lits == nil {
		g.lits = make(map[*ast.FuncLit]int)
	}
	var lits []*ast.FuncLit // the literals around the current node, innermost last
	var stack []ast.Node
	ast.Inspect(n, func(m ast.Node) bool {
		if m == nil {
			if _, ok := stack[len(stack)-1].(*ast.FuncLit); ok {
				lits = lits[:len(lits)-1]
			}
			stack = stack[:len(stack)-1]
			return true
		}
		stack = append(stack, m)
		if l, ok := m.(*ast.FuncLit); ok {
			var parent *ast.FuncLit
			if len(lits) > 0 {
				parent = lits[len(lits)-1]
			}
			counts[parent]++
			g.lits[l] = counts[parent]
			lits = append(lits, l)
		}
		return true
	})
}

// funcValue returns the C++ of the func literal fl as a func value: gp::func<Sig>(lambda).
func (fg *funcGen) funcValue(fl *ast.FuncLit) cppir.Expr {
	sig, _ := fg.g.info.TypeOf(fl).(*types.Signature)
	if sig == nil {
		fg.g.errorf(fl.Pos(), "internal error: a func literal without a signature")
		return placeholder
	}
	if fg.rawLits[fl] {
		// The argument of a macro's closure parameter (a template parameter of the instance): the
		// lambda itself, with no gp::func around it, so that nothing is allocated.
		if fg.g.awareBody(fl.Body) {
			return call("gp::aware", fg.lambda(fl))
		}
		return fg.lambda(fl)
	}
	if fg.g.awareBody(fl.Body) {
		// A func literal that calls recover: its func value is known to be aware (aware.hpp).
		return construct(fg.g.funcType(sig, fl.Pos()), call("gp::aware", fg.lambda(fl)))
	}
	return construct(fg.g.funcType(sig, fl.Pos()), fg.lambda(fl))
}

// markRawLits notes the func literals among the arguments of the call c of the instance inst that
// go to a closure parameter of a macro (genericFunc.callable): funcValue writes them as lambdas.
func (fg *funcGen) markRawLits(c *ast.CallExpr, inst *instRef) {
	if inst.gf == nil || inst.gf.callable == nil {
		return
	}
	for i, a := range c.Args {
		if i >= len(inst.gf.callable) || !inst.gf.callable[i] {
			continue
		}
		if fl, ok := ast.Unparen(a).(*ast.FuncLit); ok {
			if fg.rawLits == nil {
				fg.rawLits = make(map[*ast.FuncLit]bool)
			}
			fg.rawLits[fl] = true
		}
	}
	for _, fl := range refArgs(fg.g.info, c) {
		if fg.refLits == nil {
			fg.refLits = make(map[*ast.FuncLit]bool)
		}
		fg.refLits[fl] = true // the macro only calls it: it captures by reference
	}
}

// refArgs returns the func literals among the arguments of the call c that go to a closure parameter
// that the macro only calls (refParam): their lambdas capture the variables of the caller by
// reference, because they cannot outlive the call. The boxing analysis (box.go) uses it too, so a
// variable that only such a lambda captures and changes is not boxed.
func refArgs(info *tinfo, c *ast.CallExpr) []*ast.FuncLit {
	fun := ast.Unparen(c.Fun)
	switch x := fun.(type) {
	case *ast.IndexExpr:
		fun = ast.Unparen(x.X)
	case *ast.IndexListExpr:
		fun = ast.Unparen(x.X)
	}
	var id *ast.Ident
	switch x := fun.(type) {
	case *ast.Ident:
		id = x
	case *ast.SelectorExpr:
		id = x.Sel
	default:
		return nil
	}
	fn, _ := info.Uses[id].(*types.Func)
	if fn == nil {
		return nil
	}
	params := fn.Signature().Params()
	var out []*ast.FuncLit
	for i, a := range c.Args {
		if i >= params.Len() {
			break
		}
		if fl, ok := ast.Unparen(a).(*ast.FuncLit); ok && refParam(params.At(i).Type()) {
			out = append(out, fl)
		}
	}
	return out
}

// lambda returns the C++ lambda of the func literal fl (see the top of the file).
func (fg *funcGen) lambda(fl *ast.FuncLit) *cppir.Lambda {
	g := fg.g
	sig, _ := g.info.TypeOf(fl).(*types.Signature)
	n := g.lits[fl]
	if sig == nil || n == 0 {
		g.errorf(fl.Pos(), "internal error: a func literal that was not prepared")
		return &cppir.Lambda{Body: &cppir.Block{Inline: true}}
	}
	caps := fg.captures(fl) // the names outside the literal
	fg.noteClosure(fl, n)   // why.go
	fg.names.openScope()    // the lambda's parameters, results and outermost declarations
	f := &funcInfo{sig: sig, ftype: fl.Type, body: fl.Body, pos: fl.Pos(), top: fg.top, names: fg.names, boxed: fg.boxed}
	f.signature(g)
	lam := &cppir.Lambda{
		Captures: caps,
		Template: []cppir.Param{{Type: typeName("int"), Default: lit(strconv.Itoa(n))}},
		Params:   f.params,
		Specs:    []string{"mutable"},
		Result:   f.result,
		// A literal on one .gp line stays on one C++ line.
		Body: &cppir.Block{Inline: g.fset.Position(fl.Pos()).Line == g.fset.Position(fl.End()).Line},
	}
	child := newFuncGen(g, fg.names, f)
	child.push(&lam.Body.Stmts, &lam.Body.End)
	child.funcBody()
	child.pop(fl.Body.Rbrace)
	fg.names.closeScope()
	return lam
}

// captures returns the captures of the lambda of the func literal fl: a copy of each variable of
// the functions around fl that fl uses (a boxed one is its box), in the order of their first
// uses.
func (fg *funcGen) captures(fl *ast.FuncLit) []cppir.Capture {
	// A lambda that a macro only calls captures a variable that lives on the stack by reference (a
	// boxed one is its box, a copy of the pointer).
	return fg.captureVars([]ast.Node{fl.Body}, func(pos token.Pos) bool { return inside(fl, pos) }, fg.refLits[fl])
}

// captureNames returns the captures of a closure whose code is the nodes: a copy of each variable
// that they use and that is declared where in says it is not (the function around the closure),
// in the order of their first uses. The nodes of a range over a func are its body and the targets
// of its range variables (rangefunc.go).
func (fg *funcGen) captureNames(nodes []ast.Node, in func(pos token.Pos) bool) []cppir.Capture {
	return fg.captureVars(nodes, in, false)
}

// captureVars is captureNames, with the variables that are not boxed captured by reference if byRef.
func (fg *funcGen) captureVars(nodes []ast.Node, in func(pos token.Pos) bool, byRef bool) []cppir.Capture {
	var caps []cppir.Capture
	seen := make(map[string]bool)
	for _, node := range nodes {
		ast.Inspect(node, func(n ast.Node) bool {
			id, ok := n.(*ast.Ident)
			if !ok {
				return true
			}
			v, ok := fg.g.info.Uses[id].(*types.Var)
			if !ok || v.IsField() || isPkgVar(v) || in(v.Pos()) {
				return true
			}
			name, ok := fg.names.names[v]
			if !ok {
				fg.g.errorf(id.Pos(), "internal error: no C++ name for the captured %s", v.Name())
				return true
			}
			if !seen[name] {
				seen[name] = true
				caps = append(caps, cppir.Capture{Name: name, ByRef: byRef && !fg.isBoxed(v)})
			}
			return true
		})
	}
	return caps
}

// funcRef returns the C++ of the function fn of the package as a func value:
// gp::func_of<&gpkg_main::f>(), which needs no allocation.
func (fg *funcGen) funcRef(fn *types.Func, pos token.Pos) cppir.Expr {
	addr := &cppir.Unary{Op: "&", X: cxxName(fg.funcName(fn, pos))}
	if fg.g.awareFunc(fn) {
		return templateCall("gp::func_of_aware", []cppir.Node{addr}) // recover.go
	}
	return templateCall("gp::func_of", []cppir.Node{addr})
}

// funcValueCall reports whether the call c calls a func value: a variable or a field of func
// type, a func literal, the result of a call, an element; not a function of the package, a
// builtin, a conversion or a function of another package.
func (fg *funcGen) funcValueCall(c *ast.CallExpr) bool {
	if fg.instRef(c.Fun) != nil {
		return false // an instance of a generic function (generics.go)
	}
	if fg.g.info.Types[c.Fun].IsType() || fg.isWeakMake(c) {
		return false
	}
	switch fun := ast.Unparen(c.Fun).(type) {
	case *ast.Ident:
		_, ok := fg.g.info.Uses[fun].(*types.Var)
		return ok
	case *ast.SelectorExpr:
		sel := fg.g.info.Selections[fun]
		if sel == nil {
			_, ok := fg.g.qualifiedObj(fun).(*types.Var) // pkg.F, F a variable of func type of another package
			return ok
		}
		return sel.Kind() == types.FieldVal || sel.Kind() == types.MethodExpr // T.M(x): methods.go
	}
	return isFunc(fg.g.info.TypeOf(c.Fun))
}

// stableCallee reports whether nothing can change or free the func value e while a call of it
// runs: a func literal, the result of a call (a temporary), a local variable that isn't boxed or
// a field or an array element in one (box.go's root). For any other e the call is e.gp_call(...).
func (fg *funcGen) stableCallee(e ast.Expr) bool {
	e = ast.Unparen(e)
	if _, ok := fg.temps[e]; ok {
		return true
	}
	switch x := e.(type) {
	case *ast.FuncLit:
		return true
	case *ast.CallExpr:
		return !fg.g.info.Types[x.Fun].IsType()
	case *ast.SelectorExpr:
		if s := fg.g.info.Selections[x]; s != nil && s.Kind() == types.MethodExpr {
			return true // T.M: a lambda (methods.go)
		}
	}
	v := (&boxer{info: fg.g.info}).root(e)
	return v != nil && !fg.isBoxed(v)
}

// callValue returns the C++ of the call c of a func value (funcValueCall).
func (fg *funcGen) callValue(c *ast.CallExpr) cppir.Expr {
	sig, _ := under(fg.g.info.TypeOf(c.Fun)).(*types.Signature)
	if sig == nil {
		fg.g.errorf(c.Pos(), "internal error: a call of a func value without a signature")
		return placeholder
	}
	fun := ast.Unparen(c.Fun)
	fg.g.noteCallValue(c.Pos(), fg.stableCallee(fun)) // why.go
	var callee cppir.Expr
	if fl, ok := fun.(*ast.FuncLit); ok {
		// Called at once: no func value. A lambda over several lines is called as
		// [...]() { ... }.operator()(...): clang++ gives a call of a lambda the line where the
		// lambda starts, but a member call the line of the member, the last one, where Go
		// puts the call too (the caller's line in a stack trace, and for gdb).
		lam := fg.lambda(fl)
		callee = lam
		if !lam.Body.Inline {
			callee = &cppir.Member{X: lam, Name: "operator()"}
			lam.TailAtEnd = true
		}
	} else if x, s := fg.methodExpr(fun); x != nil {
		callee = fg.methodLambda(x, s) // T.M(x, a): its lambda, called at once (methods.go)
	} else {
		callee = fg.expr(fun)
	}
	if len(c.Args) == 1 && isTuple(fg.g.info.TypeOf(c.Args[0])) {
		// f(g()), where g has several results: std::apply(f, g()), or with a lambda that
		// converts the results or makes the variadic slice (tupleArgs, named.go). A func value
		// that could change is called with gp_call, or copied first: the copy keeps the closure
		// alive.
		if !fg.stableCallee(fun) {
			callee = construct(fg.g.funcType(sig, c.Pos()), callee)
		}
		arg := fg.expr(c.Args[0])
		if x := fg.tupleArgs(c, sig, callee, true); x != nil {
			callee = x
		}
		return &cppir.Call{Fun: cxxName("std::apply"), Args: []cppir.Expr{callee, arg}}
	}
	args := fg.callArgs(c, sig)
	if !fg.stableCallee(fun) {
		callee = &cppir.Member{X: callee, Name: "gp_call"}
	}
	return &cppir.Call{Fun: callee, Args: args}
}

// callArgs returns the C++ of the arguments of the call c of a function of the signature sig
// (a function of the package or a func value): for a variadic function, the arguments of its
// last parameter are a slice (see the top of the file).
func (fg *funcGen) callArgs(c *ast.CallExpr, sig *types.Signature) []cppir.Expr {
	if !sig.Variadic() {
		return fg.exprs(c.Args, false)
	}
	n := sig.Params().Len()
	args := fg.exprs(c.Args[:n-1], false)
	last := sig.Params().At(n - 1).Type()
	st, _ := under(last).(*types.Slice)
	switch {
	case st == nil:
		fg.g.errorf(c.Pos(), "internal error: a variadic parameter that is not a slice")
		return args
	case c.Ellipsis.IsValid():
		// f(s...): the slice itself; go/types leaves a nil s untyped.
		if a := c.Args[n-1]; fg.g.info.Types[a].IsNil() {
			return append(args, construct(fg.g.cxxType(last, a.Pos())))
		}
		return append(args, fg.expr(c.Args[n-1]))
	case len(c.Args) < n:
		return append(args, construct(fg.g.cxxType(last, c.Pos()))) // no arguments: nil
	}
	elem := fg.g.cxxType(st.Elem(), c.Pos())
	vals := fg.exprs(c.Args[n-1:], false)
	if nm, ok := types.Unalias(st.Elem()).(*types.Named); ok && fg.g.cxxNamed[nm].open && !fg.g.curOpen[nm] {
		// The element type is a template parameter that C++ deduces from the arguments of the call
		// (generics.go): it is not in scope here, so the first argument says what it is.
		elem = typeName("std::decay_t<decltype(" + cppir.ExprString(vals[0]) + ")>")
	}
	return append(args, templateCall("gp::slice_of", []cppir.Node{elem}, &cppir.InitList{Elems: vals}))
}

// deferValue returns gp_df.gp_push_call(f, args...) for defer f(args) of a func value f
// (funcValueCall): f and the arguments are evaluated now and stored in the frame. A deferred func
// literal is stored as its lambda, so its frame in a stack trace is the literal's (main.f.func1).
func (fg *funcGen) deferValue(c *ast.CallExpr) cppir.Expr {
	sig, _ := under(fg.g.info.TypeOf(c.Fun)).(*types.Signature)
	if sig == nil {
		fg.g.errorf(c.Pos(), "internal error: a deferred func value without a signature")
		return placeholder
	}
	var f cppir.Expr
	push := "gp_push_call"
	if fl, ok := ast.Unparen(c.Fun).(*ast.FuncLit); ok {
		f = fg.lambda(fl)
		if fg.g.awareBody(fl.Body) {
			push = "gp_push_call_aware" // the literal calls recover (recover.go)
		}
	} else {
		f = fg.exactExpr(c.Fun) // a copy, gp::func (the frame looks its code up: aware.hpp)
	}
	if len(c.Args) == 1 && isTuple(fg.g.info.TypeOf(c.Args[0])) {
		// defer f(g()): g's results are stored as one tuple, which a generic lambda (the trace
		// leaves it out) spreads over f's parameters when the call runs (converted, or into the
		// variadic slice: tupleArgs).
		auto := &cppir.Ref{Elem: typeName("auto"), RValue: true}
		var spread cppir.Expr = cxxName("gp_f")
		if x := fg.tupleArgs(c, sig, spread, true); x != nil {
			spread = x
		}
		apply := &cppir.Lambda{
			Params: []cppir.Param{{Type: auto, Name: "gp_f"}, {Type: auto, Name: "gp_a"}},
			Body:   &cppir.Block{Inline: true, Stmts: []cppir.Stmt{&cppir.ExprStmt{X: call("std::apply", spread, cxxName("gp_a"))}}},
		}
		return fg.pushCall("gp_push_call", apply, f, fg.exactExpr(c.Args[0]))
	}
	return fg.pushCall(push, append([]cppir.Expr{f}, fg.callArgs(c, sig)...)...)
}

// funcName returns the C++ name of the function fn qualified with its package's namespace: the name
// of an address &gpkg_main::f, or of a function of another package, ::gpkg_geom::F.
func (fg *funcGen) funcName(fn *types.Func, pos token.Pos) string {
	name := fg.cname(fn, pos)
	if !strings.HasPrefix(name, "::") && !strings.HasPrefix(name, fg.g.ns+"::") {
		name = fg.g.ns + "::" + name
	}
	return name
}
