package lower

import (
	"go/ast"
	"go/token"
	"go/types"
	"strconv"

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
//	                                          } catch (gp::panic_error& gp_e) {
//	                                              gp_df.gp_on_panic(gp_e);
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
	// Only a Go panic: any other C++ exception (a forced unwind) goes on untouched (defer.hpp).
	onPanic := &cppir.ExprStmt{Pos: end, X: frameCall("gp_on_panic", cxxName("gp_e"))}
	param := &cppir.Param{Type: &cppir.Ref{Elem: typeName("gp::panic_error")}, Name: "gp_e"}
	d.try.Catches = []*cppir.Catch{{Pos: end, Param: param, Body: &cppir.Block{Stmts: []cppir.Stmt{onPanic}, End: end}}}
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
			values[i] = cxxName(n)
			if fg.isBoxed(res.At(i)) {
				values[i] = &cppir.Paren{X: deref(values[i])}
			}
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
		if fg.isBoxed(res.At(i)) {
			targets[i].x, targets[i].mem = &cppir.Paren{X: deref(cxxName(n))}, true
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
	p := fg.g.pos(s.Pos())
	c := s.Call
	pre := fg.order(false, c) // the arguments' order, as for the call as a statement
	var push cppir.Expr
	switch fn := fg.deferredFunc(c); {
	case fn != nil:
		args := append([]cppir.Expr{cxxName(fg.cname(fn, c.Fun.Pos()))}, fg.exprs(c.Args, false)...)
		push = frameCall("gp_push", args...)
	case fg.funcValueCall(c):
		push = fg.deferValue(c) // closures.go
	default:
		push = fg.deferThunk(c)
	}
	fg.add(withPre(p, pre, &cppir.ExprStmt{Pos: p, X: push}))
}

// deferredFunc returns the function of the package that the deferred call c calls with a
// value for each of its parameters (so not f(g()) with several results of g, which goes
// through std::apply, nor a variadic function), or nil.
func (fg *funcGen) deferredFunc(c *ast.CallExpr) *types.Func {
	id, ok := ast.Unparen(c.Fun).(*ast.Ident)
	if !ok {
		return nil
	}
	fn, ok := fg.g.info.Uses[id].(*types.Func)
	if !ok || fn.Signature().Variadic() || fn.Pkg() != fg.g.pkg.Types {
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
	for _, a := range c.Args {
		if fg.constValue(a) != nil || panics && fg.staticConst(a) != nil {
			continue // the same value in the call (a constant for panic: panicid.go)
		}
		values = append(values, fg.exactExpr(a))
		name := "gp_a" + strconv.Itoa(len(params))
		params = append(params, cppir.Param{Type: &cppir.Ref{Elem: typeName("auto"), RValue: true}, Name: name})
		if t, ok := fg.temps[a]; ok {
			saved[a] = t
		}
		fg.temps[a] = name
	}
	body := fg.expr(c)
	for _, a := range c.Args {
		if t, ok := saved[a]; ok {
			fg.temps[a] = t
		} else {
			delete(fg.temps, a)
		}
	}
	if len(params) == 0 {
		// Still a generic lambda, which the trace leaves out: [](auto&&...) { ... }.
		params = []cppir.Param{{Type: &cppir.Ref{Elem: typeName("auto"), RValue: true}, Pack: true}}
	}
	lambda := &cppir.Lambda{Params: params, Body: &cppir.Block{Stmts: []cppir.Stmt{&cppir.ExprStmt{X: body}}, Inline: true}}
	return frameCall("gp_push_call", append([]cppir.Expr{lambda}, values...)...)
}
