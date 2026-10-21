package lower

import (
	"go/ast"
	"go/types"
	"strconv"

	"github.com/huuhait/gocpp/internal/cppir"
)

// concreteIfaceType proves the dynamic type of a direct conversion, or of a local interface
// whose sole definition converts a concrete value and whose uses are all method calls.
// It never substitutes the initializer:
// the actual interface still owns its original value or pointer while the call runs.
func (fg *funcGen) concreteIfaceType(e ast.Expr) types.Type {
	e = ast.Unparen(e)
	if id, ok := e.(*ast.Ident); ok {
		v, ok := fg.g.info.Uses[id].(*types.Var)
		if !ok || fg.memoryVar(v) || !fg.onlyIfaceCalls(v) {
			return nil
		}
		e = fg.singleDef(v)
		if e == nil {
			return nil
		}
		e = ast.Unparen(e)
	}
	if c, ok := e.(*ast.CallExpr); ok && fg.g.info.Types[c.Fun].IsType() &&
		isInterface(fg.g.info.TypeOf(c)) && len(c.Args) == 1 {
		e = c.Args[0]
	}
	t := fg.g.info.TypeOf(e)
	if t == nil || isInterface(t) || isNilType(t) || hasC(t) {
		return nil
	}
	return t
}

// onlyIfaceCalls rules out exposing even a Go interface to an opaque C++ reference
// parameter. Restricting every use to a method receiver needs no foreign alias analysis.
func (fg *funcGen) onlyIfaceCalls(v *types.Var) bool {
	if fg.ifaceCallUses == nil {
		fg.ifaceCallUses = make(map[*types.Var]bool)
		receivers := make(map[*ast.Ident]bool)
		ast.Inspect(fg.top, func(n ast.Node) bool {
			c, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			x, ok := ast.Unparen(c.Fun).(*ast.SelectorExpr)
			if !ok || fg.g.info.Selections[x] == nil {
				return true
			}
			if id, ok := ast.Unparen(x.X).(*ast.Ident); ok {
				receivers[id] = true
			}
			return true
		})
		ast.Inspect(fg.top, func(n ast.Node) bool {
			id, ok := n.(*ast.Ident)
			if !ok {
				return true
			}
			local, ok := fg.g.info.Uses[id].(*types.Var)
			if !ok || !isInterface(local.Type()) {
				return true
			}
			previous, seen := fg.ifaceCallUses[local]
			fg.ifaceCallUses[local] = (!seen || previous) && receivers[id]
			return true
		})
	}
	return fg.ifaceCallUses[v]
}

// devirtTarget is also used by the order pass: the direct adapter takes its receiver as
// an argument, rather than the first-evaluated object of a C++ member call.
// Promoted, foreign and generic methods, aliases to interface storage, tuple arguments and
// go/defer calls deliberately keep the dynamic path.
func (fg *funcGen) devirtTarget(c *ast.CallExpr, x *ast.SelectorExpr,
	r recvPath) (*namedDesc, methodEntry, bool) {
	if fg.fn == nil || fg.fn.decl == nil || fg.fn.decl.Type.TypeParams != nil ||
		fg.fn.sig.RecvTypeParams().Len() != 0 || fg.g.inTemplate != 0 ||
		fg.thunk != 0 || c == fg.deferCall || len(r.path) != 0 || !isInterface(r.t) ||
		len(c.Args) == 1 && isTuple(fg.g.info.TypeOf(c.Args[0])) {
		return nil, methodEntry{}, false
	}
	if m := fg.fn.method; m != nil && m.si != nil && m.si.inst != nil {
		return nil, methodEntry{}, false
	}
	t := fg.concreteIfaceType(x.X)
	ptr := false
	if p, ok := types.Unalias(t).(*types.Pointer); ok {
		t = p.Elem()
		ptr = true
	}
	named, ok := types.Unalias(t).(*types.Named)
	if !ok || named.TypeParams().Len() != 0 || isInterface(named) {
		return nil, methodEntry{}, false
	}
	si := fg.g.structOf(named, x.Pos())
	if si == nil || si.owner != fg.g {
		return nil, methodEntry{}, false
	}
	ifaceMethod := fg.g.info.Selections[x].Obj().(*types.Func)
	if hasC(ifaceMethod.Signature()) {
		return nil, methodEntry{}, false
	}
	for _, method := range fg.g.methodSet(named, ptr, x.Pos()) {
		if !method.promoted && method.fn.Id() == ifaceMethod.Id() &&
			types.Identical(method.fn.Signature(), ifaceMethod.Signature()) {
			return &namedDesc{t: named, ty: named, si: si}, method, true
		}
	}
	return nil, methodEntry{}, false
}

// devirtCall bypasses only dispatch, not the interface conversion or its ownership. The
// inline adapter is the ordinary method-table thunk: all arguments arrive before unboxing,
// and value-method nil wrappers and the borrowed/owning receiver choice stay unchanged.
func (fg *funcGen) devirtCall(c *ast.CallExpr, x *ast.SelectorExpr, r recvPath,
	recv cppir.Expr, args []cppir.Expr) cppir.Expr {
	nd, method, ok := fg.devirtTarget(c, x, r)
	if !ok {
		return nil
	}
	fg.g.noteDevirtCall(method.fn.Name(), c.Pos()) // why.go
	ptr := isPointer(fg.concreteIfaceType(x.X))
	thunk := fg.g.thunkFunc(nd, method, ptr)

	// Preserve icall's recover-ticket handling, including calls made while unwinding.
	var slot int
	for i, m := range sortedMethods(r.t.Underlying().(*types.Interface)) {
		if m.Id() == method.fn.Id() {
			slot = i
			break
		}
	}
	tab := call("gp_self.gp_tab")
	ticket := &cppir.ExprStmt{X: call("gp::detail::icall_ticket", deref(tab),
		lit(strconv.Itoa(slot)))}
	thunk.Body.Stmts = append([]cppir.Stmt{ticket}, thunk.Body.Stmts...)
	// A zero template tag identifies compiler helpers, not Go closures, in stack traces.
	adapter := &cppir.Lambda{
		Template: []cppir.Param{{Type: typeName("int"), Default: lit("0")}},
		Params:   thunk.Params,
		Result:   thunk.Result,
		Body:     thunk.Body,
	}
	return &cppir.Call{Fun: &cppir.Paren{X: adapter},
		Args: append([]cppir.Expr{recv}, args...)}
}
