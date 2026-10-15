package lower

import (
	"go/ast"
	"go/token"
	"go/types"
)

// Locals that gc copies in place as arguments of fmt (order.go: copied), though closures
// change them (the manager's decision in job 3.15b; divergence D25 in docs/spec/language.md).
//
// gc copies an argument of fmt's ...any that needs an address (convCopied) into a temporary in
// its place among the calls also when it is a local variable (walk/order.go: addrTemp, since
// Go 1.22, issue 63332): a local of the function (not a parameter or a result), whose address is
// not taken, of a type that gc keeps in registers (ssa.CanSSA). A plain C++ local can't change
// during its statement, so that copy can't be seen; but a closure can change the local:
//
//	ok := false
//	set := func() bool { ok = true; return ok }
//	fmt.Println(ok, set()) // Go: false true
//
// gc reads the old ok because it inlines the closure everywhere, so ok stays a plain local. A
// local that a closure captures by reference, because the closure is a value somewhere (stored,
// passed, deferred, or only made: _ = set) or because gc doesn't inline it, is on the heap or
// address-taken in gc, and is read after the calls, with the new value. gocpp has no escape
// analysis and no inliner, so it approximates gc: a func literal doesn't escape when its value is
// only called in the function that makes it, either at once (func() { ... }()), or through a
// local variable that the literal initializes and that is only ever called (f(): never assigned
// again, stored, passed, returned, deferred, used as a value, or used in another func literal;
// a func literal called at once counts as the function's own code, as gc inlines it), and gc
// can inline it at all: its own body has no defer or go statement and no recover call
// (notInlined). A local that box.go boxes only because closures capture and change it
// (boxCaptured), of a type that convCopied and canSSA accept, and that only such closures
// capture, is copied in place like gc does (also as a key of a slow map call: ssaKey), and so is a part of it whose type convCopied accepts
// (outerVar: t.ok, a[0].ok); any other boxed local is read after the calls. gc can differ
// (divergence D25 in docs/spec/language.md): it doesn't inline a closure that costs more than 160
// nodes (800 when it is called once; 40 inside a function of more than 5,000 nodes) and then
// reads the new value, and its escape analysis may keep a local for a closure that is stored but
// doesn't escape.

// inPlaceLocals returns the local variables of the code top (a function declaration, or a
// package variable's initializer) that an argument of fmt copies in place (above). boxed is
// box.go's analysis of top. It also returns the parameters, receivers and results in top.
func inPlaceLocals(info *types.Info, top ast.Node, boxed map[*types.Var]boxReason) (in, params map[*types.Var]bool) {
	params = make(map[*types.Var]bool) // the parameters, receivers and results in top
	addParams := func(l *ast.FieldList) {
		if l == nil {
			return
		}
		for _, f := range l.List {
			for _, id := range f.Names {
				if v, ok := info.Defs[id].(*types.Var); ok {
					params[v] = true
				}
			}
		}
	}
	escapes := make(map[*ast.FuncLit]bool)
	holders := make(map[*types.Var][]*ast.FuncLit) // the locals that a func literal is assigned to
	captures := make(map[*types.Var][]*ast.FuncLit)
	litOf := make(map[*types.Var]*ast.FuncLit) // the func literal that declares a local (nil: top)
	var lits []*ast.FuncLit                    // the func literals around the node, innermost last
	var stack []ast.Node
	atOnce := make(map[*ast.FuncLit]bool) // called where they are made: func() { ... }()
	// innermost returns the func literal that the node is in, not counting those called at once
	// (gc inlines them: their code is the function's), or nil for top's own code.
	innermost := func() *ast.FuncLit {
		for i := len(lits) - 1; i >= 0; i-- {
			if !atOnce[lits[i]] {
				return lits[i]
			}
		}
		return nil
	}
	ast.Inspect(top, func(n ast.Node) bool {
		if n == nil {
			if _, ok := stack[len(stack)-1].(*ast.FuncLit); ok {
				lits = lits[:len(lits)-1]
			}
			stack = stack[:len(stack)-1]
			return true
		}
		var parent ast.Node
		if len(stack) > 0 {
			parent = stack[len(stack)-1]
		}
		stack = append(stack, n)
		switch n := n.(type) {
		case *ast.FuncDecl:
			addParams(n.Recv)
			addParams(n.Type.Params)
			addParams(n.Type.Results)
		case *ast.FuncLit:
			addParams(n.Type.Params)
			addParams(n.Type.Results)
			if notInlined(info, n) {
				escapes[n] = true // gc captures by reference what it can't inline
			} else if v := litHolder(info, stack); v != nil {
				holders[v] = append(holders[v], n)
			} else if c, ok := parent.(*ast.CallExpr); ok && ast.Unparen(c.Fun) == n && !deferredOrGo(stack[:len(stack)-1]) {
				atOnce[n] = true
			} else {
				escapes[n] = true // stored, passed, returned or deferred
			}
			lits = append(lits, n)
		case *ast.Ident:
			v, ok := info.Defs[n].(*types.Var)
			if ok && !v.IsField() {
				litOf[v] = innermost()
			}
			if v, ok := info.Uses[n].(*types.Var); ok && !v.IsField() {
				for _, l := range lits {
					if !inside(l, v.Pos()) {
						captures[v] = append(captures[v], l)
					}
				}
			}
		}
		return true
	})
	// A local that holds a func literal must only be called, in the function that declares it.
	called := make(map[*types.Var]bool)
	for v := range holders {
		called[v] = boxed[v] == 0
	}
	stack = stack[:0]
	lits = lits[:0]
	ast.Inspect(top, func(n ast.Node) bool {
		if n == nil {
			if _, ok := stack[len(stack)-1].(*ast.FuncLit); ok {
				lits = lits[:len(lits)-1]
			}
			stack = stack[:len(stack)-1]
			return true
		}
		stack = append(stack, n)
		switch n := n.(type) {
		case *ast.FuncLit:
			lits = append(lits, n)
		case *ast.Ident:
			v, ok := info.Uses[n].(*types.Var)
			if !ok || !called[v] {
				break
			}
			if innermost() != litOf[v] {
				called[v] = false // used in another func literal
				break
			}
			if !calledAt(stack) { // also an assignment: gc can't inline the calls then
				called[v] = false
			}
		}
		return true
	})
	for v, ls := range holders {
		if !called[v] {
			for _, l := range ls {
				escapes[l] = true
			}
		}
	}
	in = make(map[*types.Var]bool)
	for v, r := range boxed {
		if r != boxCaptured || params[v] || !canSSA(v.Type()) {
			continue
		}
		ok := true
		for _, l := range captures[v] {
			if escapes[l] {
				ok = false
			}
		}
		in[v] = ok
	}
	return in, params
}

// notInlined reports whether gc never inlines the func literal l: its own body (not that of a
// func literal inside it) has a defer or go statement, or calls recover (inl.go: hairyVisitor,
// "unhandled op DEFER", "call to recover").
func notInlined(info *types.Info, l *ast.FuncLit) bool {
	found := false
	ast.Inspect(l.Body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.DeferStmt, *ast.GoStmt:
			found = true
		case *ast.CallExpr:
			if id, ok := ast.Unparen(n.Fun).(*ast.Ident); ok {
				if b, ok := info.Uses[id].(*types.Builtin); ok && b.Name() == "recover" {
					found = true
				}
			}
		}
		return !found
	})
	return found
}

// litHolder returns the local variable that the func literal at the top of the stack is
// declared with, x := func() {...} or var x = func() {...}, or nil.
func litHolder(info *types.Info, stack []ast.Node) *types.Var {
	lit := stack[len(stack)-1]
	if len(stack) < 2 {
		return nil
	}
	var lhs []*ast.Ident
	var rhs []ast.Expr
	switch p := stack[len(stack)-2].(type) {
	case *ast.AssignStmt:
		if p.Tok != token.DEFINE {
			return nil // x = func() {...}: x may hold other funcs too
		}
		for _, e := range p.Lhs {
			id, _ := e.(*ast.Ident)
			lhs = append(lhs, id)
		}
		rhs = p.Rhs
	case *ast.ValueSpec:
		lhs, rhs = p.Names, p.Values
	}
	if len(lhs) != len(rhs) {
		return nil
	}
	for i, e := range rhs {
		if e != lit || lhs[i] == nil {
			continue
		}
		obj := info.Defs[lhs[i]]
		if obj == nil {
			obj = info.Uses[lhs[i]]
		}
		if v, ok := obj.(*types.Var); ok && v.Parent() != v.Pkg().Scope() {
			return v
		}
	}
	return nil
}

// calledAt reports whether the identifier at the top of the stack is the function of a call
// that is not deferred: f(...).
func calledAt(stack []ast.Node) bool {
	id := stack[len(stack)-1]
	for i := len(stack) - 2; i >= 0; i-- {
		switch p := stack[i].(type) {
		case *ast.ParenExpr:
			id = p
			continue
		case *ast.CallExpr:
			return p.Fun == id && !deferredOrGo(stack[:i+1])
		}
		return false
	}
	return false
}

// deferredOrGo reports whether the call at the top of the stack is the call of a defer or go
// statement.
func deferredOrGo(stack []ast.Node) bool {
	if len(stack) < 2 {
		return false
	}
	switch stack[len(stack)-2].(type) {
	case *ast.DeferStmt, *ast.GoStmt:
		return true
	}
	return false
}

// canSSA reports whether gc keeps a value of type t in registers (ssa.CanSSA): at most 4
// words (in Go's sizes), and an array of at most one element or a struct of at most 4 fields,
// whose parts are too.
func canSSA(t types.Type) bool {
	if goSizes.Sizeof(t) > 4*8 {
		return false
	}
	switch u := under(t).(type) {
	case *types.Array:
		return u.Len() <= 1 && canSSA(u.Elem())
	case *types.Struct:
		if u.NumFields() > 4 {
			return false
		}
		for i := range u.NumFields() {
			if !canSSA(u.Field(i).Type()) {
				return false
			}
		}
	}
	return true
}

// inPlaceLocal reports whether v is a boxed local that an argument of fmt copies in place.
func (fg *funcGen) inPlaceLocal(v *types.Var) bool {
	return convCopied(v.Type()) && fg.ssaLocal(v)
}

// ssaLocal reports whether v is a boxed local that gc keeps in registers (above, without
// convCopied: inPlaceLocals), which its addrTemp copies in place: as an argument of fmt that
// needs an address (inPlaceLocal) and as a key of a slow map call (orderKey: ssaKey).
func (fg *funcGen) ssaLocal(v *types.Var) bool {
	if !fg.isBoxed(v) || fg.top == nil {
		return false
	}
	fg.findInPlace()
	return fg.inPlace[v]
}

// ssaKey reports whether gc's addrTemp copies the addressable key k of a slow map call in place
// (orderKey): k is a local that gc keeps in registers or a part of one (outerVar), which in
// gocpp is a boxed local that only closures that gc inlines change (ssaLocal), or a plain local
// read through an index that a call can change (varIndex; as inPlacePart, without convCopied).
func (fg *funcGen) ssaKey(k ast.Expr) bool {
	v := fg.outerVar(k)
	if v == nil || fg.top == nil {
		return false
	}
	if fg.isBoxed(v) {
		return fg.ssaLocal(v)
	}
	if v.Parent() == nil || v.Parent() == fg.g.pkg.Types.Scope() || !canSSA(v.Type()) || !fg.varIndex(k) {
		return false
	}
	fg.findInPlace()
	return !fg.inPlaceParams[v]
}

// findInPlace runs inPlaceLocals for fg.top, once (gp_var_init changes top for each package
// variable).
func (fg *funcGen) findInPlace() {
	if fg.inPlace == nil || fg.inPlaceTop != fg.top {
		fg.inPlace, fg.inPlaceParams = inPlaceLocals(fg.g.info, fg.top, fg.boxed)
		fg.inPlaceTop = fg.top
	}
}

// inPlacePart reports whether gc copies x, an addressable part of a variable (a field, an
// element of an array: outerVar), in place as an argument of fmt (order.go: copied). gc copies it
// when its own type needs an address (convCopied) and the variable is a local that it copies in
// place: a boxed one by inPlaceLocal's rule, and a plain local of the function (not a parameter or
// a result) whose type gc keeps in registers (canSSA), boxed or not in gocpp. For a plain local
// the copy can be seen only through an index that a call changes (one[k] of a [1]T, where
// fmt.Println(one[k], setk()) indexes with the old k, as in Go), so only such a part is copied.
func (fg *funcGen) inPlacePart(x ast.Expr) bool {
	v := fg.outerVar(x)
	if v == nil || fg.top == nil || !convCopied(fg.g.info.TypeOf(x)) {
		return false
	}
	if fg.isBoxed(v) {
		return fg.inPlaceLocal(v)
	}
	if v.Parent() == nil || v.Parent() == fg.g.pkg.Types.Scope() || !canSSA(v.Type()) || !fg.varIndex(x) {
		return false
	}
	fg.findInPlace()
	return !fg.inPlaceParams[v]
}

// varIndex reports whether the part x of a variable (outerVar) has an index that is not a
// constant on its way.
func (fg *funcGen) varIndex(x ast.Expr) bool {
	for {
		switch e := ast.Unparen(x).(type) {
		case *ast.SelectorExpr:
			x = e.X
		case *ast.IndexExpr:
			if fg.constValue(e.Index) == nil {
				return true
			}
			x = e.X
		default:
			return false
		}
	}
}

// outerVar returns the variable that e is a part of, stored in it: e itself, a field of a struct
// value (not through a pointer), an element of an array value; or nil (gc's ir.OuterValue).
func (fg *funcGen) outerVar(e ast.Expr) *types.Var {
	for {
		switch x := ast.Unparen(e).(type) {
		case *ast.SelectorExpr:
			sel := fg.g.info.Selections[x]
			if sel == nil || sel.Kind() != types.FieldVal || sel.Indirect() {
				return nil
			}
			e = x.X
		case *ast.IndexExpr:
			if !isArray(fg.g.info.TypeOf(x.X)) {
				return nil
			}
			e = x.X
		case *ast.Ident:
			v, _ := fg.g.info.Uses[x].(*types.Var)
			return v
		default:
			return nil
		}
	}
}
