package lower

import (
	"go/ast"
	"go/token"
	"go/types"
	"strconv"
	"strings"

	"github.com/huuhait/gocpp/internal/cppir"
)

// defer (Phase 03 README §2.8, step 3.13): the single-exit shape.
//
// A function with a defer statement (main and the init functions too) gets a gp::defer_frame
// (runtime/include/gocpp/defer.hpp), its body goes into a try block, and every way out of the
// body goes through the frame's gp_run, which runs the deferred calls, the last one first, and
// then throws again the panic that left the body, if any:
//
//	func f(n int) int {                   int64_t f(int64_t n) {
//		defer g(n)                            int64_t gp_r0{};
//		if n > 0 {                            gp::defer_frame gp_df; try {
//			return n                              gp_df.gp_push(g, n);
//		}                                         if (n > 0) {
//		return 0                                      gp_r0 = n; goto gp_ret0;
//	}                                             }
//	                                              gp_r0 = 0; goto gp_ret1;
//	                                          } catch (...) {
//	                                              gp_df.gp_on_exception();
//	                                          }
//	                                          gp_df.gp_run(); return gp_r0;
//	                                      gp_ret0:; gp_df.gp_run(); return gp_r0;
//	                                      gp_ret1:; gp_df.gp_run(); return gp_r0;
//	                                      }
//
//   - The results are variables, declared before the frame: the named results keep their names
//     (a boxed one is a box), the others are gp_r0, gp_r1, ... A return assigns them (bind, with
//     Go's order) and jumps to its own exit, after the try block: Go sets the results first, then
//     runs the deferred calls, so a deferred call that changes a named result through a pointer
//     (defer set(&err)) changes what the function returns. A goto out of a try block is allowed,
//     and it jumps over no declaration: the body is in the try block, a scope of its own.
//   - Each return has its own exit, at the return's line, as Go's deferreturn is: a panic in a
//     deferred call shows the function at that line in the trace, and gdb's next stays on it.
//     The end of the body (and a panic, after the catch) goes to the exit at the closing brace.
//   - Functions without defer stay as they are.
//
// defer f(a, b) evaluates f and its arguments at the defer statement, in Go's order: they are
// ordered exactly like the call f(a, b) as a statement (gc's order for ODEFER copies the
// arguments after the calls of the statement, as for a call: defer show(z, inc(&z)) shows the
// new z), and stored in the frame:
//   - a function of the package: gp_df.gp_push(f, a, b), which converts the arguments to f's
//     parameter types there and stores them with the function pointer;
//   - anything else (a builtin, fmt.Println): gp_df.gp_push_call with a generic lambda whose
//     body is the call, with each argument that is not a constant a parameter of the lambda:
//     gp_df.gp_push_call([](auto&& gp_a0) { gp::println(gp_a0, 1); }, x). The stack trace leaves
//     a generic lambda out (gocpp's helper): it is no func literal (those have their numbers,
//     main.f.func1, as a template argument: closures.go);
//   - a func value (step 3.11, closures.go: deferValue): gp_df.gp_push_call(f, a, b), a copy of
//     the func value f, evaluated now (a nil f panics when the call runs, as in Go), and a
//     deferred func literal as its lambda, gp_df.gp_push_call([n]<int = 1>() mutable -> void {
//     ... }): it is the user's code, which the trace shows as main.f.func1, as Go does. A func
//     literal with defers of its own gets the same shape inside its lambda.
//
// The generic lambda is one line, at the defer statement's line, so gdb has no lines of its own
// to step through there; a deferred function of the program is stepped into as any call.
//
// A method call (step 3.10) stores its receiver as the lambda's first argument, evaluated at
// the defer statement as Go does: a copy of the value for a value method (defer p.M() copies
// *p there, and panics there for a nil p), the pointer for a pointer method (methods.go:
// storeRecv).

// A deferInfo is what the single-exit shape of a function with defers needs (funcInfo.defers).
type deferInfo struct {
	try   *cppir.Try
	exits []deferExit // one per return statement, in source order
}

// A deferExit is the exit of a return: its label and the place of the return.
type deferExit struct {
	label string
	pos   cppir.Pos
}

// The C++ names of the frame (gp_ names: no user name can be one, names.go).
const deferFrame = "gp_df"

// hasDefer reports whether the body has a defer statement of its own (not one of a function
// literal in it).
func hasDefer(body *ast.BlockStmt) bool {
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		switch n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.DeferStmt:
			found = true
		}
		return !found
	})
	return found
}

// prepareDefers is prepare's part for a function with defers: it gets a deferInfo, and its
// results that have no names get gp_r names, for the variables that the returns assign.
func (f *funcInfo) prepareDefers() {
	if f.body == nil || !hasDefer(f.body) {
		return
	}
	f.defers = &deferInfo{}
	res := f.sig.Results()
	if f.results == nil && res.Len() > 0 {
		f.results = make([]string, res.Len())
		for i := range f.results {
			f.results[i] = f.names.temp("r")
		}
	}
}

// pushCall returns the call that stores a deferred call: gp_df.name(args...) for a defer statement,
// and for a go statement the call that starts the goroutine: gp::go_fn for gp_push, gp::go_call
// for gp_push_call (the aware and interface variants of the frame's functions make no
// difference in a new goroutine: nothing is panicking there, so recover finds no ticket).
func (fg *funcGen) pushCall(name string, args ...cppir.Expr) cppir.Expr {
	if !fg.spawn {
		return frameCall(name, args...)
	}
	if strings.HasPrefix(name, "gp_push_call") {
		return call("gp::go_call", args...)
	}
	return call("gp::go_fn", args...)
}

// frameCall returns gp_df.name(args...).
func frameCall(name string, args ...cppir.Expr) cppir.Expr {
	return &cppir.Call{Fun: &cppir.Member{X: cxxName(deferFrame), Name: name}, Args: args}
}

// openDefers starts the single-exit shape of a function with defers, after the declarations
// of its results: the frame, and the try block, which then gets the body. It does nothing for
// a function without defers.
func (fg *funcGen) openDefers() {
	d := fg.fn.defers
	if d == nil {
		return
	}
	p := fg.g.pos(fg.fn.pos)
	d.try = &cppir.Try{Pos: p, Body: &cppir.Block{}}
	frame := varDecl(p, typeName("gp::defer_frame"), deferFrame, nil)
	fg.add(&cppir.Line{Pos: p, Stmts: []cppir.Stmt{frame, d.try}})
	fg.push(&d.try.Body.Stmts, &d.try.Body.End)
	fg.names.openScope()
	fg.fresh = true
}

// closeDefers ends the shape that openDefers started, at the body's closing brace rbrace: the
// catch, the exit of the end of the body, and the exits of the returns.
func (fg *funcGen) closeDefers(rbrace token.Pos) {
	d := fg.fn.defers
	if d == nil {
		return
	}
	fg.names.closeScope()
	fg.pop(rbrace) // the try block: its } is the catch's line
	end := fg.g.pos(rbrace)
	// A panic, Goexit or a C++ exception: gp_on_exception keeps it for gp_run, and throws a forced
	// unwind (glibc's thread cancellation) again at once (defer.hpp).
	onPanic := &cppir.ExprStmt{Pos: end, X: frameCall("gp_on_exception")}
	d.try.Catches = []*cppir.Catch{{Pos: end, Body: &cppir.Block{Stmts: []cppir.Stmt{onPanic}, End: end}}}
	// The end of the body, and a panic: gp_run throws it again. A void function without
	// returns needs no return statement there.
	fg.add(&cppir.Line{Pos: end, Stmts: fg.exitStmts(end, len(d.exits) > 0 || len(fg.fn.results) > 0)})
	for _, e := range d.exits {
		stmts := append([]cppir.Stmt{&cppir.Label{Pos: e.pos, Name: e.label}}, fg.exitStmts(e.pos, true)...)
		fg.add(&cppir.Line{Pos: e.pos, Stmts: stmts})
	}
	fg.fresh = false
}

// exitStmts returns an exit: gp_df.gp_run(); and, with ret, the return of the results.
func (fg *funcGen) exitStmts(pos cppir.Pos, ret bool) []cppir.Stmt {
	stmts := []cppir.Stmt{&cppir.ExprStmt{Pos: pos, X: frameCall("gp_run")}}
	if !ret {
		return stmts
	}
	r := &cppir.Return{Pos: pos}
	if len(fg.fn.results) > 0 {
		res := fg.fn.sig.Results()
		values := make([]cppir.Expr, len(fg.fn.results))
		for i, n := range fg.fn.results {
			values[i] = fg.resultValue(res.At(i), n)
		}
		r.X = braceList(values)
	}
	return append(stmts, r)
}

// deferReturn lowers a return in a function with defers: it assigns the results, as an
// assignment to them does (bind: all values first, in Go's order), and jumps to its exit.
func (fg *funcGen) deferReturn(s *ast.ReturnStmt) {
	d := fg.fn.defers
	p := fg.g.pos(s.Pos())
	label := fg.names.temp("ret")
	d.exits = append(d.exits, deferExit{label, p})
	jump := &cppir.Goto{Pos: p, Label: label}
	if len(s.Results) == 0 {
		fg.add(jump) // a bare return: the named results are set already
		return
	}
	res := fg.fn.sig.Results()
	targets := make([]target, len(fg.fn.results))
	for i, n := range fg.fn.results {
		targets[i] = varTarget(n)
		targets[i].gtype = res.At(i).Type() // which return f() converts to (step 3.9)
		targets[i].gcDefer = true           // gc's ascompatee: a result, with defers
		targets[i].ret = true               // a C/C++ local that is returned moves (returnMove)
		if fg.isBoxed(res.At(i)) {
			targets[i].x = &cppir.Paren{X: deref(cxxName(n))}
		}
	}
	var lines []cppir.Stmt
	fg.push(&lines, nil)
	fg.bind(s.Pos(), targets, nil, s.Results, true)
	fg.pop(s.Pos())
	if len(lines) == 1 {
		if l, ok := lines[0].(*cppir.Line); ok {
			lines = l.Stmts
		}
	}
	fg.add(&cppir.Line{Pos: p, Stmts: append(lines, jump)})
}

// deferStmt lowers defer f(args): the deferred call goes into the frame, with f and the
// arguments evaluated now (see the top of the file).
func (fg *funcGen) deferStmt(s *ast.DeferStmt) {
	fg.deferredCall(s.Pos(), s.Call, false)
}

// goStmt lowers go f(args): the same evaluation as defer, f and the arguments now, in the
// calling goroutine, and the call on a new one (go.go): gp::go_fn(f, args...) for a function of
// the package and gp::go_call(callable, args...) for the rest, the counterparts of the frame's
// gp_push and gp_push_call.
func (fg *funcGen) goStmt(s *ast.GoStmt) {
	fg.deferredCall(s.Pos(), s.Call, true)
}

// deferredCall lowers the call c of a defer statement (spawn false) or of a go statement (spawn
// true) at pos: its function and arguments are evaluated here, and stored for the call that
// comes later (the end of the function) or elsewhere (a new goroutine).
func (fg *funcGen) deferredCall(pos token.Pos, c *ast.CallExpr, spawn bool) {
	p := fg.g.pos(pos)
	defer func(old bool) { fg.spawn = old }(fg.spawn)
	fg.spawn = spawn
	// The arguments' order, as for the call as a statement, except that gc evaluates the func
	// value and the receiver first (its normalizeGoDeferCall copies them, then the arguments,
	// into new variables, in order), so a func value isn't late and a receiver is an argument
	// of gp_push_call, not the object of a member call (order.go).
	fg.deferCall = c
	pre := fg.order(false, c)
	fg.deferCall = nil
	var push cppir.Expr
	switch fn := fg.deferredFunc(c); {
	case fn != nil:
		args := []cppir.Expr{cxxName(fg.cname(fn, c.Fun.Pos()))}
		for _, a := range c.Args {
			if fg.g.pkg.Opaque.IsOpaque(a) && fg.constValue(a) == nil {
				args = append(args, fg.deferArg(a)) // a C/C++ value: copied or moved into the frame (cbox.go)
			} else {
				args = append(args, fg.expr(a))
			}
		}
		push = fg.pushCall(pushName("gp_push", fg.deferKind(c)), args...) // gp_push_aware: recover.go
	case fg.isBuiltin(c, "recover"):
		// defer recover() does nothing: recover is the deferred call itself, not called by one
		// (recover.go), and it has no arguments.
		auto := &cppir.Ref{Elem: typeName("auto"), RValue: true}
		push = fg.pushCall("gp_push_call", &cppir.Lambda{
			Params: []cppir.Param{{Type: auto, Pack: true}},
			Body:   &cppir.Block{Inline: true},
		})
	case fg.g.opaqueCallee(c):
		push = fg.deferThunk(c) // a member of a C/C++ object, or a C++ callable (cbox.go)
	case fg.funcValueCall(c):
		push = fg.deferValue(c) // closures.go
	default:
		push = fg.deferThunk(c)
	}
	fg.add(withPre(p, pre, &cppir.ExprStmt{Pos: p, X: push}))
}

// deferredFunc returns the function of the program that the deferred call c calls with a
// value for each of its parameters (so not f(g()) with several results of g, which goes
// through std::apply, nor a variadic function), or nil.
func (fg *funcGen) deferredFunc(c *ast.CallExpr) *types.Func {
	var fn *types.Func
	if fg.instRef(c.Fun) != nil {
		return nil // an instance of a generic function (generics.go)
	}
	switch fun := ast.Unparen(c.Fun).(type) {
	case *ast.Ident:
		fn, _ = fg.g.info.Uses[fun].(*types.Func)
	case *ast.SelectorExpr:
		fn = fg.g.qualifiedFunc(fun) // pkg.F, F a function of another package of the program
	}
	if fn == nil || fn.Signature().Variadic() || fg.g.other(fn.Pkg()) == nil {
		return nil
	}
	if len(c.Args) == 1 && isTuple(fg.g.info.TypeOf(c.Args[0])) {
		return nil
	}
	return fn
}

// deferThunk returns gp_df.gp_push_call(lambda, args...) for the deferred call c: the
// arguments that are not constants are evaluated here and stored, and the lambda makes the
// call with them, through the usual translation of the call (fg.call), which writes each
// stored argument as the lambda's parameter (fg.temps).
func (fg *funcGen) deferThunk(c *ast.CallExpr) cppir.Expr {
	var params []cppir.Param
	var values []cppir.Expr
	saved := make(map[ast.Expr]string)
	panics := fg.isBuiltin(c, "panic")
	// A method's receiver is stored too, as the lambda's first parameter (methods.go).
	defer fg.storeRecv(c, &params, &values)()
	// The object of a member of a C/C++ value, or the C++ callable (cbox.go).
	defer fg.storeObject(c, &params, &values)()
	for _, a := range c.Args {
		if fg.constValue(a) != nil || panics && fg.staticConst(a) != nil {
			continue // the same value in the call (a constant for panic: panicid.go)
		}
		held, ptrs := fg.heldBoxes, fg.heldPtrs // the argument is evaluated here, not in the thunk: the variable itself
		fg.heldBoxes, fg.heldPtrs = nil, nil
		if fg.isCallback(a) && fg.cTarget(c) {
			values = append(values, fg.callbackValue(a)) // a func for a deferred C/C++ call (ccallback.go)
		} else {
			values = append(values, fg.deferArg(a))
		}
		fg.heldBoxes, fg.heldPtrs = held, ptrs
		name := "gp_a" + strconv.Itoa(len(params))
		params = append(params, cppir.Param{Type: &cppir.Ref{Elem: typeName("auto"), RValue: true}, Name: name})
		if t, ok := fg.temps[a]; ok {
			saved[a] = t
		}
		fg.temps[a] = name
		if _, ok := ast.Unparen(a).(*ast.CallExpr); ok && fg.g.pkg.Opaque.IsOpaque(a) {
			// A temporary or std.move(x): the frame's copy is used once, and the call takes it by
			// move (cMoved), as it would take the temporary; a variable's copy is an lvalue, as
			// Go's deferred call gets its own variable.
			fg.markAuto(name)
		}
	}
	body := fg.expr(c)
	for _, a := range c.Args {
		if t, ok := saved[a]; ok {
			fg.temps[a] = t
		} else {
			delete(fg.temps, a)
		}
	}
	// A C/C++ variable whose address an argument takes (defer lib.close(&v)) is kept alive by the
	// frame: it holds the box too, as a parameter that the call doesn't use.
	for _, b := range fg.keptBoxes(c) {
		values = append(values, cxxName(fg.cname(b, c.Pos())))
		params = append(params, cppir.Param{Type: &cppir.Ref{Elem: typeName("auto"), RValue: true}})
	}
	if len(params) == 0 {
		// Still a generic lambda, which the trace leaves out: [](auto&&...) { ... }.
		params = []cppir.Param{{Type: &cppir.Ref{Elem: typeName("auto"), RValue: true}, Pack: true}}
	}
	lambda := &cppir.Lambda{Params: params, Body: &cppir.Block{Stmts: []cppir.Stmt{&cppir.ExprStmt{X: body}}, Inline: true}}
	// A method that is aware, or called through an interface, needs the ticket (recover.go).
	return fg.pushCall(pushName("gp_push_call", fg.deferKind(c)), append([]cppir.Expr{lambda}, values...)...)
}
