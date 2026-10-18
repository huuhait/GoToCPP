package lower

import (
	"go/ast"
	"go/types"

	"github.com/huuhait/gocpp/internal/cppir"
)

// recover (Phase 05 step 5.10, runtime/include/gocpp/aware.hpp).
//
// Go's recover stops a panic only when it is called directly by a deferred function: the function
// that contains the call must be the function the defer machinery called. gocpp calls a function
// that contains recover() an *aware* function:
//
//   - its first statement takes the ticket that the defer machinery leaves for the call it makes
//     (null for every other call), `gp::defer_ctx gp_dc = gp::take_defer_ctx();`, and a recover() in
//     its own body is `gp::recover(gp_dc)`: nil without a ticket, the panic's value with one;
//   - where the defer machinery calls it, the emitter says so: gp_df.gp_push_aware(f, ...) for a
//     function of the package, gp_push_call_aware(lambda, ...) for a deferred func literal or a
//     method, gp_push_call_icall for a method called through an interface (the runtime looks at
//     the method's thunk then), and a func value (gp::func) is looked up when it is called: a func
//     literal that contains recover is made as gp::func<Sig>(gp::aware(lambda)), a function of
//     the package as gp::func_of_aware<&f>().
//
// A function without recover() emits exactly what it did before (the only difference is in the
// functions with a defer: the catch clause, defer.go).

// awareBody reports whether the body of a function or func literal calls recover itself (not
// in a func literal inside it, which has its own body): the function is aware. A deferred
// `defer recover()` doesn't count: Go's recover does nothing there (the builtin is the deferred
// call, not a function that a deferred function calls).
func (g *gen) awareBody(body *ast.BlockStmt) bool {
	if body == nil {
		return false
	}
	if v, ok := g.awareBodies[body]; ok {
		return v
	}
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.DeferStmt:
			// The deferred call itself is no recover of its function's; its arguments are
			// evaluated now, so a recover() there counts.
			if id, ok := ast.Unparen(n.Call.Fun).(*ast.Ident); ok {
				if b, ok := g.info.Uses[id].(*types.Builtin); ok && b.Name() == "recover" {
					return false
				}
			}
		case *ast.CallExpr:
			if id, ok := ast.Unparen(n.Fun).(*ast.Ident); ok {
				if b, ok := g.info.Uses[id].(*types.Builtin); ok && b.Name() == "recover" {
					found = true
				}
			}
		}
		return !found
	})
	if g.awareBodies == nil {
		g.awareBodies = make(map[*ast.BlockStmt]bool)
	}
	g.awareBodies[body] = found
	return found
}

// awareFunc reports whether the function or method fn of the package is aware.
func (g *gen) awareFunc(fn *types.Func) bool {
	if fn == nil {
		return false
	}
	if fn.Pkg() != g.pkg.Types {
		// A function of another package: that package's lowering knows its bodies.
		if o := g.foreign(fn.Pkg()); o != nil {
			return o.awareFunc(fn)
		}
		return false
	}
	if g.funcDecls == nil {
		g.funcDecls = make(map[*types.Func]*ast.FuncDecl)
		for _, f := range g.pkg.Load.Files {
			for _, d := range f.AST.Decls {
				if d, ok := d.(*ast.FuncDecl); ok && d.Body != nil {
					if obj, ok := g.info.Defs[d.Name].(*types.Func); ok {
						g.funcDecls[obj] = d
					}
				}
			}
		}
	}
	d := g.funcDecls[fn.Origin()]
	return d != nil && g.awareBody(d.Body)
}

// deferCtxName is the C++ name of the ticket variable of an aware function.
const deferCtxName = "gp_dc"

// takeTicket is the first statement of an aware function: gp::defer_ctx gp_dc = gp::take_defer_ctx();
func (fg *funcGen) takeTicket() {
	if !fg.g.awareBody(fg.fn.body) {
		return
	}
	fg.add(varDecl(fg.g.pos(fg.fn.pos), typeName("gp::defer_ctx"), deferCtxName, call("gp::take_defer_ctx")))
}

// recoverCall returns the C++ of the call of the builtin recover: gp::recover(gp_dc), or nil
// outside a function.
func (fg *funcGen) recoverCall() cppir.Expr {
	if fg.fn == nil || !fg.g.awareBody(fg.fn.body) {
		// Outside a function body (a package variable's initializer): no deferred call called it,
		// so Go gives nil.
		return construct(typeName("gp::any"))
	}
	return call("gp::recover", cxxName(deferCtxName))
}

// deferKind says how the defer machinery must call the deferred call c: "" (a plain call),
// "aware" (the callee is an aware function of the package: it takes the ticket) or "icall" (the
// callee is a method called through an interface: the runtime decides from its thunk).
func (fg *funcGen) deferKind(c *ast.CallExpr) string {
	if r := fg.instRef(c.Fun); r != nil {
		// An instance of a generic function, F[int](x) or inferred: it is aware if the generic
		// function calls recover (generics.go).
		if r.fn != nil && fg.g.awareFunc(r.fn) {
			return "aware"
		}
		return ""
	}
	switch fun := ast.Unparen(c.Fun).(type) {
	case *ast.Ident:
		if fn, ok := fg.g.info.Uses[fun].(*types.Func); ok && fg.g.awareFunc(fn) {
			return "aware"
		}
	case *ast.SelectorExpr:
		sel := fg.g.info.Selections[fun]
		if sel == nil {
			if fn := fg.g.qualifiedFunc(fun); fn != nil && fg.g.awareFunc(fn) {
				return "aware" // pkg.F: F is a function of another package that calls recover
			}
			return ""
		}
		if sel.Kind() != types.MethodVal {
			return ""
		}
		fn, _ := sel.Obj().(*types.Func)
		if fn == nil {
			return ""
		}
		if isIfaceMethod(fn) {
			return "icall"
		}
		if fg.g.awareFunc(fn) {
			return "aware"
		}
	}
	return ""
}

// pushName returns the name of the frame's gp_push function for the base name (gp_push or
// gp_push_call) and the kind of deferKind.
func pushName(base, kind string) string {
	if kind == "" {
		return base
	}
	return base + "_" + kind
}

// isIfaceMethod reports whether fn is a method of an interface type.
func isIfaceMethod(fn *types.Func) bool {
	if fn == nil {
		return false
	}
	recv := fn.Signature().Recv()
	return recv != nil && types.IsInterface(recv.Type())
}
