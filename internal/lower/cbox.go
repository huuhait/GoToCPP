package lower

import (
	"go/ast"
	"go/token"
	"go/types"
	"slices"
	"strconv"

	"github.com/huuhait/gocpp/internal/cppir"
	"github.com/huuhait/gocpp/internal/interop"
)

// Boxes of C/C++ objects (Phase 04 README §2.5, step 4.8; runtime: gocpp/interop.hpp).
//
// A C/C++ object can be changed by anything that has its address: a member call (v.push_back),
// an operator, a function that takes a reference (std.swap(v, w)). So the Phase 03 rule "a
// variable that is never assigned is captured by copy" is wrong for it, and gocpp can't see
// what a C++ call does. An opaque local (a variable whose type holds a C/C++ object by value:
// cObject) that a func literal uses, or that a deferred call is made on, is therefore always
// boxed, whatever the closure does with it. The closure copies the box (a gp::ptr<T>), and the
// closure and the function share the one object, as they share a Go variable. The same box makes
// the object outlive the block that declares it, which `defer v.clear()` needs: C++ destroys a
// local at the end of its block, before the function's deferred calls run.
//
// Capture by copy would be right only for a type that can be copied, and whether a C++ type can
// be copied is the C++ compiler's to say, not gocpp's. So a closure never captures an opaque local
// by copy: the policy table in docs/spec/interop.md ("Types that can't be copied") lists what
// each construct does.
//
// The box of a C/C++ object is not gp::new_<T>(value), which would move the value into the box (a
// type that can't be moved, std::mutex, couldn't be boxed): the object is built in the box from
// the prvalue of its initializer (gp::cgo::box, guaranteed copy elision).
//
//	v := std.vector[int]{}                auto v = gp::cgo::box([&] { return ::std::vector<int64_t>{}; });
//	var m std.mutex                       gp::ptr<::std::mutex> m = gp::cgo::new_box<::std::mutex>();
//	var n C.int = f()                     gp::ptr<int> n = gp::cgo::box_as<int>([&]() -> int { return ::f(); });

// cObject reports whether a variable of type t holds a C/C++ object by value: a C/C++ type, a
// raw C/C++ pointer (*lib.FILE: the pointer is the object), a struct that has one in a field, or
// an array of them. A slice, a map, a channel and a func hold their elements by reference, and a
// pointer to a gocpp type is a gocpp pointer: they follow Phase 03's rules.
func cObject(t types.Type) bool {
	return cObjectSeen(t, make(map[types.Type]bool))
}

func cObjectSeen(t types.Type, seen map[types.Type]bool) bool {
	if t == nil || seen[t] {
		return false
	}
	seen[t] = true
	switch u := t.Underlying().(type) {
	case *types.Basic:
		return u.Kind() == types.Invalid
	case *types.Pointer:
		return !complete(u.Elem())
	case *types.Struct:
		for i := range u.NumFields() {
			if cObjectSeen(u.Field(i).Type(), seen) {
				return true
			}
		}
	case *types.Array:
		return cObjectSeen(u.Elem(), seen)
	}
	return false
}

// isCBox reports whether the variable v lives in a box of a C/C++ object (cBox).
func (fg *funcGen) isCBox(v *types.Var) bool {
	return fg.isBoxed(v) && cObject(v.Type())
}

// isAutoType reports whether typ is auto: the type of a variable declared from a C/C++ value
// without a type (x := lib.f()).
func isAutoType(typ cppir.Type) bool {
	n, ok := typ.(*cppir.TypeName)
	return ok && n.Name == "auto" && len(n.Args) == 0
}

// cBox returns the box of a variable of the C/C++ type typ (auto if gocpp doesn't know it) with
// the value: gp::cgo::box_as<T>([&]() -> T { return value; }), or gp::cgo::box([&] { return
// value; }) for auto, which builds the object in the box from the prvalue. The zero value (a nil
// value) is gp::cgo::new_box<T>().
func cBox(typ cppir.Type, value cppir.Expr) cppir.Expr {
	if value == nil {
		return templateCall("gp::cgo::new_box", []cppir.Node{typ})
	}
	auto := isAutoType(typ)
	lam := cBoxLambda(typ, value)
	if auto {
		return call("gp::cgo::box", lam)
	}
	return templateCall("gp::cgo::box_as", []cppir.Node{typ}, lam)
}

// cBoxLambda returns the lambda [&]() -> T { return value; } that builds the object of cBox in place
// (no result type for auto).
func cBoxLambda(typ cppir.Type, value cppir.Expr) *cppir.Lambda {
	auto := isAutoType(typ)
	if l, ok := value.(*cppir.InitList); ok && l.Type == nil && !auto {
		value = &cppir.InitList{Type: typ, Elems: l.Elems} // T{a, b}: explicit constructors too
	}
	lam := &cppir.Lambda{
		Default: "&",
		Body:    &cppir.Block{Inline: true, Stmts: []cppir.Stmt{&cppir.Return{X: value}}},
	}
	if !auto {
		lam.Result = typ
	}
	return lam
}

// cBoxDecl returns the declaration of the variable name that is the box of a C/C++ object of the
// type typ, with the value (nil: the zero value): auto for a type gocpp doesn't know.
func cBoxDecl(pos cppir.Pos, typ cppir.Type, name string, value cppir.Expr) cppir.Stmt {
	var t cppir.Type = ptrType(typ)
	if isAutoType(typ) {
		t = typ
	}
	return varDecl(pos, t, name, cBox(typ, value))
}

// boxType returns the C++ type of the variable of the box of a C/C++ object whose type is typ:
// gp::ptr<T>, or auto.
func boxType(typ cppir.Type) cppir.Type {
	if isAutoType(typ) {
		return typ
	}
	return ptrType(typ)
}

// boxDeferred marks the variables that a deferred call needs in a box (boxer in box.go): every
// C/C++ local that the call's object or arguments mention (the object of a member call, defer
// v.clear(), the path of defer h.v.clear(), a local in an argument, defer lib.f(v.data()): a
// pointer or reference into it is stored in the frame), except what an argument copies: a bare
// variable, and a value of a type that has no C/C++ part (defer fmt.Println(int(v.size()))) or a
// conversion to a C number (C.int(n)), which can't refer to a local. The frame holds the boxes
// (deferThunk), because C++ destroys a local when its block ends, which is before the function's
// deferred calls run.
func (b *boxer) boxDeferred(c *ast.CallExpr) {
	for _, v := range deferLocals(b.info, c) {
		b.mark(v, boxDefer)
	}
}

// deferLocals returns the C/C++ locals that the deferred call c keeps alive (boxDeferred), in the
// order of their first mention. In the object (the function of the call) every mention counts. In
// an argument only a path counts, whose pointer or reference stays reachable: &v, v.f, v.data(),
// v[i], *v. A plain use of a variable (n in Vec{x: n}, f(n), n + 1) copies its value.
func deferLocals(info *types.Info, c *ast.CallExpr) []*types.Var {
	b := &boxer{info: info}
	var out []*types.Var
	add := func(id *ast.Ident) {
		if v := b.localVar(id); v != nil && cObject(v.Type()) && !slices.Contains(out, v) {
			out = append(out, v)
		}
	}
	ast.Inspect(c.Fun, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.FuncLit:
			return false // (its captures are boxed already)
		case *ast.Ident:
			add(x)
		}
		return true
	})
	for _, a := range c.Args {
		if copiedArg(info, a) {
			continue
		}
		ast.Inspect(a, func(n ast.Node) bool {
			var path ast.Expr
			switch x := n.(type) {
			case *ast.FuncLit:
				return false
			case *ast.SelectorExpr, *ast.IndexExpr, *ast.SliceExpr, *ast.StarExpr:
				path = x.(ast.Expr)
			case *ast.UnaryExpr:
				if x.Op == token.AND {
					path = x
				}
			}
			for path != nil {
				switch y := ast.Unparen(path).(type) {
				case *ast.Ident:
					add(y)
					path = nil
				case *ast.SelectorExpr:
					path = y.X
				case *ast.IndexExpr:
					path = y.X
				case *ast.SliceExpr:
					path = y.X
				case *ast.StarExpr:
					path = y.X
				case *ast.UnaryExpr:
					path = y.X
				default:
					path = nil
				}
			}
			return true
		})
	}
	return out
}

// copiedArg reports whether the argument a of a deferred call is evaluated and copied into the
// frame without anything of a C/C++ local staying reachable: a bare variable, a value whose Go type
// has no C/C++ part (int(v.size()), a string), and a conversion to a C number (C.int(n)).
func copiedArg(info *types.Info, a ast.Expr) bool {
	a = ast.Unparen(a)
	if _, ok := a.(*ast.Ident); ok {
		return true
	}
	if tv, ok := info.Types[a]; ok && tv.IsValue() && tv.Type != nil && complete(tv.Type) {
		return true
	}
	if c, ok := a.(*ast.CallExpr); ok && len(c.Args) == 1 {
		if sel, ok := ast.Unparen(c.Fun).(*ast.SelectorExpr); ok {
			if id, ok := sel.X.(*ast.Ident); ok && id.Name == "C" && cBuiltins[sel.Sel.Name] != "" {
				return true
			}
		}
	}
	return false
}

// addrHeld returns the C/C++ locals of the function top that the frame of their own function must
// hold: a local whose address is taken (&x, a raw C++ address that a pointer, a slice, a map or a
// struct can keep) in a function that has a defer statement of its own and declares it, and
// whose pointer can outlive its C++ block:
//   - the local is declared in the function's own block (not in an inner block or a loop): C++
//     destroys it before the deferred calls run, where Go keeps it until the function ends;
//   - or the address, or a variable that holds it (a slice it was appended to, a struct that
//     has it in a field, a map), is stored in a variable declared outside the local's block, in a
//     package variable or in a field of one: the pointer is visibly kept past the block.
//
// The frame holds the box (gp_hold) until after every deferred call ran. A pointer that is only
// passed to a call, used as the receiver of a call or stored in a variable of the same block
// dies with the block, as in C++ (so a lock guard whose address is passed to a function is
// destroyed at the end of its iteration or block, as without defers), and one that a C++ call
// keeps or gives back (lib.find(x)) follows C++'s lifetime rules.
func addrHeld(info *types.Info, top ast.Node) map[*types.Var]bool {
	type fn struct {
		pos, end token.Pos
		defers   bool
		scope    *types.Scope // the scope of the parameters and the function's own block
	}
	var fns []fn
	ast.Inspect(top, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.FuncDecl:
			if x.Body != nil {
				fns = append(fns, fn{x.Pos(), x.End(), hasDefer(x.Body), info.Scopes[x.Type]})
			}
		case *ast.FuncLit:
			fns = append(fns, fn{x.Pos(), x.End(), hasDefer(x.Body), info.Scopes[x.Type]})
		}
		return true
	})
	b := &boxer{info: info}
	// the locals whose address is taken
	var taken []*types.Var
	ast.Inspect(top, func(n ast.Node) bool {
		if u, ok := n.(*ast.UnaryExpr); ok && u.Op == token.AND && cAddr(info, u) {
			if v := b.localVar(rootIdentOf(u.X)); v != nil && !slices.Contains(taken, v) {
				taken = append(taken, v)
			}
		}
		return true
	})
	if len(taken) == 0 {
		return nil
	}
	// stores[w]: the address-taken locals whose address is stored in the variable w (nil: in a
	// package variable or anything that is not a local)
	stores := make(map[*types.Var][]*types.Var)
	sources := func(e ast.Expr) []*types.Var {
		var out []*types.Var
		ast.Inspect(e, func(n ast.Node) bool {
			switch x := n.(type) {
			// (a func literal is read through: the pointers it captures travel with it)
			case *ast.UnaryExpr:
				if x.Op == token.AND && cAddr(info, x) {
					if v := b.localVar(rootIdentOf(x.X)); v != nil && !slices.Contains(out, v) {
						out = append(out, v)
					}
				}
			case *ast.Ident:
				if w := b.localVar(x); w != nil {
					for _, s := range stores[w] {
						if !slices.Contains(out, s) {
							out = append(out, s)
						}
					}
				}
			}
			return true
		})
		return out
	}
	store := func(lhs, rhs ast.Expr) {
		// the variable that the target is a part of: x, x.f, x[i], *x
		var base *types.Var
		e := ast.Unparen(lhs)
		if id, ok := e.(*ast.Ident); ok && id.Name == "_" {
			return // nothing keeps it
		}
	walk:
		for {
			switch y := e.(type) {
			case *ast.SelectorExpr:
				e = ast.Unparen(y.X)
			case *ast.IndexExpr:
				e = ast.Unparen(y.X)
			case *ast.StarExpr:
				e = ast.Unparen(y.X)
			case *ast.Ident:
				base = b.localVar(y)
				break walk
			default:
				break walk
			}
		}
		for _, s := range sources(rhs) {
			if !slices.Contains(stores[base], s) {
				stores[base] = append(stores[base], s)
			}
		}
	}
	for range 2 { // (a later store can feed an earlier use in a loop)
		ast.Inspect(top, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.AssignStmt:
				for i, l := range x.Lhs {
					switch {
					case len(x.Lhs) == len(x.Rhs):
						store(l, x.Rhs[i])
					case len(x.Rhs) == 1:
						store(l, x.Rhs[0])
					}
				}
			case *ast.ValueSpec:
				for i, id := range x.Names {
					switch {
					case len(x.Names) == len(x.Values):
						store(id, x.Values[i])
					case len(x.Values) == 1:
						store(id, x.Values[0])
					}
				}
			}
			return true
		})
	}
	held := make(map[*types.Var]bool)
	for _, v := range taken {
		var in *fn // the innermost function that declares v
		for i := range fns {
			if f := &fns[i]; f.pos <= v.Pos() && v.Pos() < f.end && (in == nil || f.pos >= in.pos) {
				in = f
			}
		}
		if in == nil || !in.defers {
			continue
		}
		if v.Parent() == in.scope {
			held[v] = true // declared in the function's own block
			continue
		}
		sc := v.Parent()
		for w, srcs := range stores {
			if !slices.Contains(srcs, v) {
				continue
			}
			if w == nil || sc == nil || !(sc.Pos() <= w.Pos() && w.Pos() < sc.End()) {
				held[v] = true // kept in a variable that outlives v's block
			}
		}
	}
	return held
}

// A deferObject says how a deferred call gets the C/C++ object it is made on (the object of a
// member call, defer v.clear(), or the callee that is a C++ callable, defer cb()). Go evaluates
// it when the defer statement runs; the call runs later, after the C++ blocks of the locals ended.
type deferObject int

const (
	// objStatic is an object that lives as long as the program (a C++ global, a package
	// variable, a header's name): the deferred call names it.
	objStatic deferObject = iota
	// objBoxed is a path that starts at a local variable (v, h.v, p.next): the local is
	// boxed, the frame holds its box, and the path is read when the call runs. (Go reads a
	// pointer in the path now: the difference shows only when the local is assigned before
	// the function ends; docs/spec/interop.md says so.)
	objBoxed
	// objPtr is a path that starts at a local gocpp pointer (s.c, s.mu): the frame holds a copy of
	// the pointer, as Go evaluates it now, and the path is read through it when the call runs.
	objPtr
	// objValue is a temporary (a call, a literal), or a local callable: evaluated now and
	// moved (copied for a variable) into the frame, as Go stores the func value.
	objValue
	// objBad is any other place that holds a local (v[i], *p): the call would read it late.
	objBad
)

// deferObjectOf classifies x, the object of a deferred call, and returns the local variable that
// a boxed path starts at.
func (g *gen) deferObjectOf(x ast.Expr) (deferObject, *types.Var) {
	switch y := ast.Unparen(x).(type) {
	case *ast.CallExpr, *ast.CompositeLit, *ast.FuncLit:
		return objValue, nil
	case *ast.Ident:
		if v, ok := g.info.Uses[y].(*types.Var); ok && !isPkgVar(v) && !v.IsField() {
			if !cObject(v.Type()) {
				if p, ok := v.Type().Underlying().(*types.Pointer); ok && complete(p.Elem()) {
					return objPtr, v
				}
				return objBad, nil // a gocpp value: Go reads the place now
			}
			return objBoxed, v
		}
		return objStatic, nil
	case *ast.SelectorExpr:
		if _, ok := g.cRefOf(y); ok {
			return objStatic, nil // a C/C++ name: lib.global
		}
		if g.isC(y.X) {
			return objStatic, nil
		}
		return g.deferObjectOf(y.X)
	}
	// Anything else: fine if it holds no local (lib.table[0], (*lib.p)), else the call would
	// read the local when it runs.
	bad := false
	ast.Inspect(x, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok {
			if v, ok := g.info.Uses[id].(*types.Var); ok && !isPkgVar(v) && !v.IsField() {
				bad = true
			}
		}
		return !bad
	})
	if bad {
		return objBad, nil
	}
	return objStatic, nil
}

// deferRecvOK reports whether the deferred call c can keep the object it is made on (cbox.go), and
// reports it if it can't: a receiver that is an element or a pointer's target of a local
// (defer v[i].m(), defer (*p).m()) is a place that Go evaluates at the defer statement and C++ would
// read when the call runs.
func (g *gen) deferRecvOK(c *ast.CallExpr) bool {
	if !g.opaqueCallee(c) {
		return true
	}
	obj := c.Fun
	what := "callee"
	if sel := g.opaqueMember(c.Fun); sel != nil {
		obj, what = sel.X, "object"
	}
	if k, _ := g.deferObjectOf(obj); k == objBad {
		g.userError(obj.Pos(), "cannot defer a call whose %s is %s: Go evaluates it when the defer statement runs, and a deferred C++ call can only keep a variable's object or a temporary: use a func literal, defer func() { ... }()", what, g.srcText(obj))
		return false
	}
	return true
}

// storeObject makes the thunk of the deferred call c (deferThunk) keep its object: for a member of
// a C/C++ value, the box of the local that the object path starts at (the frame holds a copy of the
// box, and the path is read through it when the call runs), or the object itself if it is a
// temporary (gp::cgo::keep: moved into the frame, or a pointer if C++ gave a reference); for a
// C++ callable, its value (a copy, as Go stores the func value). It adds the thunk's parameters and
// the stored values, and returns the function that forgets what it set up.
func (fg *funcGen) storeObject(c *ast.CallExpr, params *[]cppir.Param, values *[]cppir.Expr) func() {
	g := fg.g
	if !g.opaqueCallee(c) {
		return func() {}
	}
	param := func() string {
		name := "gp_a" + strconv.Itoa(len(*params))
		*params = append(*params, cppir.Param{Type: &cppir.Ref{Elem: typeName("auto"), RValue: true}, Name: name})
		return name
	}
	sel := g.opaqueMember(c.Fun)
	if sel == nil {
		// A C++ callable: Go evaluates the func value now.
		kind, _ := g.deferObjectOf(c.Fun)
		if kind == objStatic {
			return func() {}
		}
		*values = append(*values, fg.deferArg(c.Fun))
		name := param()
		old, had := fg.temps[c.Fun]
		fg.temps[c.Fun] = name
		return func() {
			if had {
				fg.temps[c.Fun] = old
			} else {
				delete(fg.temps, c.Fun)
			}
		}
	}
	kind, root := g.deferObjectOf(sel.X)
	switch kind {
	case objBoxed:
		if !fg.isBoxed(root) {
			g.errorf(sel.Pos(), "internal error: the object of a deferred call, %s, is not boxed", root.Name())
			return func() {}
		}
		*values = append(*values, cxxName(fg.cname(root, sel.Pos())))
		name := param()
		if fg.heldBoxes == nil {
			fg.heldBoxes = make(map[*types.Var]string)
		}
		fg.heldBoxes[root] = name
		return func() { delete(fg.heldBoxes, root) }
	case objPtr:
		var ptr cppir.Expr = cxxName(fg.cname(root, sel.Pos()))
		if fg.isBoxed(root) {
			ptr = &cppir.Paren{X: deref(ptr)}
		}
		*values = append(*values, call("gp::cgo::nonnil", ptr)) // Go panics here for a nil pointer
		name := param()
		if fg.heldPtrs == nil {
			fg.heldPtrs = make(map[*types.Var]string)
		}
		fg.heldPtrs[root] = name
		return func() { delete(fg.heldPtrs, root) }
	case objValue:
		// A temporary named by the order pass (auto&& gp_t0 = f()) is an rvalue if f gave a value
		// and an lvalue if it gave a reference, which keep() tells apart.
		val := fg.exactExpr(sel.X)
		if t := fg.temps[sel.X]; t != "" {
			val = movedName(t)
		}
		*values = append(*values, call("gp::cgo::keep", val))
		name := param()
		old, had := fg.temps[sel.X]
		fg.temps[sel.X] = name
		return func() {
			if had {
				fg.temps[sel.X] = old
			} else {
				delete(fg.temps, sel.X)
			}
		}
	}
	return func() {}
}

// deferArg returns the C++ of the argument a of a deferred call, as the frame stores it: evaluated
// now; a C/C++ value goes through gp::cgo::defer_arg, which copies a variable and moves a temporary
// (std.move(x), f()), and is a clear error for a type that can't be copied or moved. A conversion to
// a number (C.int(n)) is stored as it is.
func (fg *funcGen) deferArg(a ast.Expr) cppir.Expr {
	x := fg.exactExpr(a)
	if !fg.g.pkg.Opaque.IsOpaque(a) {
		return x
	}
	switch u := ast.Unparen(a).(type) {
	case *ast.Ident, *ast.SelectorExpr, *ast.IndexExpr, *ast.StarExpr:
		if fg.temps[ast.Unparen(a)] == "" {
			return call("gp::cgo::defer_arg", x)
		}
	case *ast.CompositeLit:
		return x // a prvalue, built where the frame takes it
	case *ast.CallExpr:
		if fg.g.pkg.Conversion(u) != nil {
			return x
		}
		if r, ok := fg.g.cRefOf(u.Fun); ok && r.kind == interop.Type {
			if n, ok := r.x.(*cppir.Name); ok && isBuiltinC(n.Name) {
				return x
			}
		}
	}
	return call("gp::cgo::defer_arg", fg.cMoved(a, x))
}

// keptBoxes returns the boxed C/C++ locals that the frame of the deferred call c must hold besides
// the object's own box (deferLocals): the thunk takes each as a parameter that it doesn't use.
func (fg *funcGen) keptBoxes(c *ast.CallExpr) []*types.Var {
	var boxes []*types.Var
	for _, v := range deferLocals(fg.g.info, c) {
		if _, held := fg.heldBoxes[v]; !held && fg.isCBox(v) {
			boxes = append(boxes, v)
		}
	}
	return boxes
}

// holdBox returns the statement that makes the frame of the function hold the box name of a C/C++
// local whose address is taken (addrHeld), or nil if v needs none: the frame keeps the box until
// every deferred call has run, however early or late each was registered.
func (fg *funcGen) holdBox(v *types.Var, name string) cppir.Stmt {
	if fg.fn == nil || fg.fn.defers == nil || fg.top == nil || !fg.isCBox(v) {
		return nil
	}
	if fg.addrHeld == nil {
		fg.addrHeld = addrHeld(fg.g.info, fg.top)
	}
	if !fg.addrHeld[v] {
		return nil
	}
	return holdStmt(name)
}

// holdStmt returns gp_df.gp_hold(name): see holdBox.
func holdStmt(name string) cppir.Stmt {
	return &cppir.ExprStmt{X: frameCall("gp_hold", cxxName(name))}
}

// isSlot reports whether the result r of the function is a gp::result_slot (result.hpp): a result
// of a C/C++ type that has no name in the program, in a function with defers, which the return
// statements set and the exit takes. A named result is a variable of its own.
func (fg *funcGen) isSlot(r *types.Var) bool {
	return fg.fn.defers != nil && cObject(r.Type()) && (r.Name() == "" || r.Name() == "_") && !fg.isBoxed(r)
}

// resultValue returns the C++ of the result variable r (named name) as the function returns it, at
// its exit or in a bare return: the variable (a box is read through, (*r)), where a C/C++ object
// is moved out of an unboxed variable, which nothing sees any more, and out of a slot; a boxed one
// is copied if C++ can copy it (the closures that share the box keep their object), else moved
// (gp::cgo::result_of).
func (fg *funcGen) resultValue(r *types.Var, name string) cppir.Expr {
	switch {
	case fg.isSlot(r):
		return &cppir.Call{Fun: &cppir.Member{X: cxxName(name), Name: "gp_take"}}
	case fg.isCBox(r):
		return call("gp::cgo::result_of", deref(cxxName(name)))
	case fg.isBoxed(r):
		return &cppir.Paren{X: deref(cxxName(name))}
	case cObject(r.Type()):
		return movedName(name)
	}
	return cxxName(name)
}

// returnMove returns the C++ x of the value e of the return statement being lowered, as the
// result is made from it: a C/C++ object in a local that nothing sees after the return is moved
// (static_cast<T&&>(v)), where Go copies: its type may not be copyable, and the copy is
// never seen. That holds for an unboxed local used once in the results (no closure or deferred
// call can see it; a result variable is not moved into itself). A boxed one (the closures that
// share the box keep their object) is copied if C++ can, else moved (gp::cgo::result_of). store is
// whether the value is stored into a result variable or a std::tuple, which C++ copies from a
// variable; the return of one value is not moved by hand, so C++ can elide the copy (NRVO).
func (fg *funcGen) returnMove(e ast.Expr, x cppir.Expr, store bool) cppir.Expr {
	id, ok := ast.Unparen(e).(*ast.Ident)
	if !ok || fg.retStmt == nil || fg.temps[ast.Unparen(e)] != "" {
		return x
	}
	v, ok := fg.g.info.Uses[id].(*types.Var)
	if !ok || isPkgVar(v) || !cObject(v.Type()) || v.IsField() {
		return x
	}
	if fg.isBoxed(v) {
		return call("gp::cgo::result_of", deref(cxxName(fg.cname(v, id.Pos()))))
	}
	if fg.isResult(v) || !store {
		return x // (the return of one value moves a local by itself, and keeps C++'s NRVO)
	}
	uses := 0
	for _, r := range fg.retStmt.Results {
		ast.Inspect(r, func(n ast.Node) bool {
			if u, ok := n.(*ast.Ident); ok && fg.g.info.Uses[u] == v {
				uses++
			}
			return true
		})
	}
	if uses > 1 {
		return x
	}
	return movedName(fg.cname(v, id.Pos()))
}

// withHold returns the declaration decl, followed by the statement hold if it is not nil (holdBox),
// as one line.
func withHold(pos cppir.Pos, decl cppir.Stmt, hold cppir.Stmt) cppir.Stmt {
	if hold == nil {
		return decl
	}
	return &cppir.Line{Pos: pos, Stmts: []cppir.Stmt{decl, hold}}
}

// maybeNoDefault reports whether the C/C++ type of the field f can be a class with no default
// constructor, which gocpp can't see: not a raw pointer or one of C's number types, which always
// have a zero value (so the struct's code carries no check for those).
func (g *gen) maybeNoDefault(f *types.Var) bool {
	switch t := ast.Unparen(g.typeExprOf(f)).(type) {
	case *ast.StarExpr:
		return false
	case *ast.SelectorExpr:
		return !(g.isC(t.X) && cBuiltins[t.Sel.Name] != "")
	}
	return true
}

// rootIdentOf returns the variable that the operand e of & starts at: x in &x, &x[i], &x.f,
// &x[i].f and &(*x).f (a conservative reading: the address of a part of x counts as the address of
// x for the hold and escape rules), or nil. (cAddr, box.go, takes the same forms for a C/C++ root.)
func rootIdentOf(e ast.Expr) *ast.Ident {
	for {
		switch y := ast.Unparen(e).(type) {
		case *ast.Ident:
			return y
		case *ast.SelectorExpr:
			e = y.X
		case *ast.IndexExpr:
			e = y.X
		case *ast.StarExpr:
			e = y.X
		default:
			return nil
		}
	}
}
