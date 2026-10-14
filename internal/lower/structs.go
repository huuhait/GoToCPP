package lower

import (
	"go/ast"
	"go/token"
	"go/types"
	"strconv"

	"github.com/huuhait/gocpp/internal/cppir"
)

// Structs (Phase 03 README §2.4, step 3.8): the expressions. typedecl.go has the C++ structs.
//
//	Go                          C++
//	var p Point, Point{}        Point p{};  Point{}  (the default member initializers: zero)
//	Point{1, 2}                 Point{1, 2}
//	Point{Y: f(), X: 1}         Point{.X = 1, .Y = f()}  (declaration order; temporaries keep Go's
//	                            order where C++'s would differ: order.go)
//	[]Point{{1, 2}}             gp::slice_of<Point>({Point{1, 2}})  (an elided type is written)
//	[]*Point{{1, 2}}, &Point{}  gp::new_<Point>(Point{1, 2})
//	p.X, pp.X (pp is *Point)    p.X, pp->X  (gp::ptr's -> panics like Go for a nil pp)
//	o.X (X promoted from an     o.Inner.X, o.Inner->X for an embedded *Inner (go/types gives
//	embedded Inner)             the path: Selection.Index)
//	&p.X, &pp.X, &s[i].X        gp::ptr<int64_t>(p, &(*p).X) (p boxed), gp::ptr<int64_t>(pp,
//	                            &pp->X), gp::ptr<int64_t>(s.gp_owner(), &s[i].X) (arrays.go: addrOf)
//	&f().X                      gp::field_ptr(f(), &Point::X): the pointer is evaluated once
//	p == q, p != q              p == q, p != q  (the generated operator==, when comparable)
//	n = n.kids[0]               n = Node(n.kids[0])  (a copy first: sharedCopy)
//
// A struct is a value: assigning, passing and returning one copies it, and a struct is a C++
// class type (a call with a struct argument is qualified, funcCallName, and a temporary is moved
// into its variable). A local struct whose field's address is taken is boxed (box.go), like an
// array; one larger than 128 KiB too, and its zero value is made in its box (D24 has the rest).
//
// Go's order of evaluation (order.go): a field is part of the variable it is in, so x.f is read
// where x is read: after the calls of its scope for a package or a boxed variable, and anywhere
// for a plain local. A field through a pointer (p.f, also an embedded pointer on the way) is a
// read of memory that a call can change and panics for a nil pointer, like *p. The values of a
// keyed literal are evaluated in source order, as gc does, while C++ evaluates the designated
// initializers in declaration order: order.go sees each value at its C++ place (litElem) and puts
// the ones that C++ would evaluate too late into temporaries. In an assignment, the pointer of a
// target p.f (and the slice and index of s[i].f) is evaluated before the values, and the store
// goes through the old pointer (saveOperands), as in Go.

// compLit returns the C++ of a composite literal. One whose type is a pointer is an element of
// an outer literal whose type is elided, {...} for &T{...} ([]*Point{{1, 2}}): a new block that
// holds the value.
func (fg *funcGen) compLit(e *ast.CompositeLit) cppir.Expr {
	t := fg.g.info.TypeOf(e)
	if p, ok := under(t).(*types.Pointer); ok {
		typ := fg.g.cxxType(p.Elem(), e.Pos())
		if largeVar(p.Elem()) {
			return largeBox(typ, fg.litValue(e, p.Elem()))
		}
		return newBox(typ, fg.litValue(e, p.Elem()))
	}
	return fg.litValue(e, t)
}

// litValue returns the C++ of the composite literal e of type t (not a pointer).
func (fg *funcGen) litValue(e *ast.CompositeLit, t types.Type) cppir.Expr {
	switch {
	case isArray(t):
		return fg.arrayLit(e) // arrays.go
	case isStruct(t):
		return fg.structLit(e, t)
	}
	return fg.compositeLit(e) // slices.go
}

// litType returns the type of the composite literal e, the T of a literal {...} whose type *T
// is elided (&T{...}).
func (g *gen) litType(e *ast.CompositeLit) types.Type {
	t := g.info.TypeOf(e)
	if p, ok := under(t).(*types.Pointer); ok {
		return p.Elem()
	}
	return t
}

// structLit returns the C++ of the struct literal e of type t: T{a, b} for a literal without
// keys, whose braced list C++ evaluates in order, or T{.X = a, .Y = b} with the keyed values in
// the fields' order, which C++'s designated initializers need. The missing fields are zero.
func (fg *funcGen) structLit(e *ast.CompositeLit, t types.Type) cppir.Expr {
	typ := fg.g.cxxType(t, e.Lbrace)
	if !hasKeys(e) && !largeVar(t) {
		return &cppir.InitList{Type: typ, Elems: fg.exprs(e.Elts, false)}
	}
	// Keyed, or larger than 128 KiB: designated initializers (also for a positional literal, in
	// the same order), which largeBox turns into stores into a new block (newStruct).
	st := under(t).(*types.Struct)
	values := make([]ast.Expr, st.NumFields())
	for i, elt := range e.Elts {
		if kv, ok := elt.(*ast.KeyValueExpr); ok {
			values[fieldIndex(st, kv.Key.(*ast.Ident).Name)] = kv.Value
		} else {
			values[i] = elt
		}
	}
	var elems []cppir.Expr
	for i, v := range values {
		if v != nil {
			elems = append(elems, &cppir.Designated{Name: fg.g.fieldName(t, i, e.Lbrace), X: fg.expr(v)})
		}
	}
	return &cppir.InitList{Type: typ, Elems: elems}
}

// newStruct returns the C++ that makes a new block for a struct of the C++ type typ that is larger
// than 128 KiB from its literal x, T{.a = v, ...} (largeBox, arrays.go), without the literal on the
// C++ stack (D24): gp::new_init<T>([&](T& gp_b) { gp_b.a = v; ... }), which makes the zero value in
// the block and then stores the values, in the fields' order, as the literal would (order.go
// puts the values that Go evaluates in another order into temporaries first: litElem). A value
// that is a literal itself is stored part by part, and an empty one not at all (storeLit).
func newStruct(typ cppir.Type, x *cppir.InitList) cppir.Expr {
	for _, el := range x.Elems {
		if _, ok := el.(*cppir.Designated); !ok {
			return newBox(typ, x) // (structLit writes a large literal designated)
		}
	}
	body := &cppir.Block{Inline: true}
	storeLit(&body.Stmts, cxxName("gp_b"), x)
	init := &cppir.Lambda{Default: "&", Params: []cppir.Param{{Type: &cppir.Ref{Elem: typ}, Name: "gp_b"}}, Body: body}
	return templateCall("gp::new_init", []cppir.Node{typ}, init)
}

// storeLit adds the stores that give dst, which is zero, the value x (newStruct), in the order
// of the literal: a struct literal field by field, an array literal (also a keyed one) element
// by element, so that no large part of the value is a temporary; a zero value (T{}, [N]T{}) is
// no store at all. Any other value is one store.
func storeLit(stmts *[]cppir.Stmt, dst, x cppir.Expr) {
	store := func(dst, x cppir.Expr) {
		*stmts = append(*stmts, &cppir.ExprStmt{X: &cppir.Assign{Op: "=", X: dst, Y: x}})
	}
	switch l := x.(type) {
	case *cppir.InitList:
		if l.Type == nil {
			break
		}
		if len(l.Elems) == 0 {
			return
		}
		if tn, ok := l.Type.(*cppir.TypeName); ok && tn.Name == "gp::array" {
			for i, e := range l.Elems {
				storeLit(stmts, &cppir.Index{X: dst, Index: lit(strconv.Itoa(i))}, e)
			}
			return
		}
		designated := true
		for _, e := range l.Elems {
			_, ok := e.(*cppir.Designated)
			designated = designated && ok
		}
		if designated {
			for _, e := range l.Elems {
				d := e.(*cppir.Designated)
				storeLit(stmts, &cppir.Member{X: dst, Name: d.Name}, d.X)
			}
			return
		}
	case *cppir.Call:
		if n, ok := l.Fun.(*cppir.Name); ok && n.Name == "gp::array_keyed" && len(l.Args) == 1 {
			if pairs, ok := l.Args[0].(*cppir.InitList); ok {
				for _, p := range pairs.Elems {
					kv := p.(*cppir.InitList).Elems
					storeLit(stmts, &cppir.Index{X: dst, Index: kv[0]}, kv[1])
				}
				return
			}
		}
	}
	store(dst, x)
}

// fieldIndex returns the index of the field name of st, or -1.
func fieldIndex(st *types.Struct, name string) int {
	for i := range st.NumFields() {
		if st.Field(i).Name() == name {
			return i
		}
	}
	return -1
}

// litElem returns the place in C++'s order of the element i of the composite literal e, from 1
// (order.go): the element's own place, but the field's for a keyed struct literal, whose
// designated initializers C++ evaluates in the fields' order.
func (fg *funcGen) litElem(e *ast.CompositeLit, i int) int {
	t := fg.g.litType(e)
	if kv, ok := e.Elts[i].(*ast.KeyValueExpr); ok && isStruct(t) {
		return fieldIndex(under(t).(*types.Struct), kv.Key.(*ast.Ident).Name) + 1
	}
	return i + 1
}

// fieldSel returns the selection of x if it selects a field (x.f, also promoted), or nil.
func (fg *funcGen) fieldSel(x *ast.SelectorExpr) *types.Selection {
	if sel := fg.g.info.Selections[x]; sel != nil && sel.Kind() == types.FieldVal {
		return sel
	}
	return nil
}

// selector returns the C++ of the selector x: a field, x.f.
func (fg *funcGen) selector(x *ast.SelectorExpr) cppir.Expr {
	sel := fg.g.info.Selections[x]
	switch {
	case sel == nil:
		fg.g.unsupported(x.Pos(), "qualified names (Phase 06)")
	case sel.Kind() == types.FieldVal:
		return fg.fieldChain(fg.expr(x.X), sel, x.Pos())
	default:
		fg.g.unsupported(x.Pos(), "methods (Phase 03 step 3.10)")
	}
	return placeholder
}

// fieldChain returns the C++ of the field that sel selects in base (the C++ of the operand):
// each step of the path (more than one for a promoted field, through its embedded fields) is
// .f, or ->f after a pointer (gp::ptr's ->, which panics like Go for nil).
func (fg *funcGen) fieldChain(base cppir.Expr, sel *types.Selection, pos token.Pos) cppir.Expr {
	x, _ := fg.fieldSteps(base, sel.Recv(), sel.Index(), pos)
	return x
}

// fieldSteps returns the C++ of the fields at the path index in base, whose Go type is t, and
// the type of the last one (fieldChain).
func (fg *funcGen) fieldSteps(base cppir.Expr, t types.Type, index []int, pos token.Pos) (cppir.Expr, types.Type) {
	x := base
	for _, i := range index {
		arrow := false
		if p, ok := under(t).(*types.Pointer); ok {
			arrow, t = true, p.Elem()
		}
		x = &cppir.Member{X: x, Arrow: arrow, Name: fg.g.fieldName(t, i, pos)}
		t = under(t).(*types.Struct).Field(i).Type()
	}
	return x, t
}

// laterPointer reports whether the path of sel goes through a pointer after its first step: an
// embedded pointer field on the way to a promoted field (o.X for an embedded *Inner).
func laterPointer(sel *types.Selection) bool {
	t := sel.Recv()
	for k, i := range sel.Index() {
		if p, ok := under(t).(*types.Pointer); ok {
			if k > 0 {
				return true
			}
			t = p.Elem()
		}
		t = under(t).(*types.Struct).Field(i).Type()
	}
	return false
}

// firstPointer reports whether the operand of sel is a pointer: p.f.
func firstPointer(sel *types.Selection) bool {
	return isPointer(sel.Recv())
}

// selectorOperands returns the operands of the target x.f of an assignment that Go evaluates
// before the values (stmt.go: targetOperands): the pointer p of p.f, or the operands of the
// place that the struct x is in (s and i of s[i].f). An embedded pointer on the path (o.X for
// an embedded *Inner, Phase 05) makes x itself the operand.
func (fg *funcGen) selectorOperands(x *ast.SelectorExpr) []ast.Expr {
	sel := fg.fieldSel(x)
	switch {
	case sel == nil:
		return nil
	case firstPointer(sel) || laterPointer(sel):
		return []ast.Expr{x.X}
	}
	return fg.targetOperands([]ast.Expr{x.X})
}

// saveField returns the C++ of the target x.f of an assignment with its operands in
// temporaries where save gives one (stmt.go: saveOperands, arrays.go: saveArrayElem), or nil if
// none needs one: p, p.f = q, 1 stores through the old p.
func (fg *funcGen) saveField(x *ast.SelectorExpr, save func(ast.Expr) cppir.Expr) cppir.Expr {
	sel := fg.fieldSel(x)
	if sel == nil {
		return nil
	}
	var base cppir.Expr
	if firstPointer(sel) {
		base = save(x.X)
	} else {
		base = fg.saveArrayElem(x.X, save)
	}
	if base == nil {
		return nil
	}
	return fg.fieldChain(base, sel, x.Pos())
}

// embeddedPointer reports whether the field x.f is reached through an embedded pointer after the
// first step of its path (o.F of an embedded *E: laterPointer).
func (fg *funcGen) embeddedPointer(x *ast.SelectorExpr) bool {
	sel := fg.fieldSel(x)
	return sel != nil && laterPointer(sel)
}

// embeddedPtr returns the C++ of the last embedded pointer on the path of the field x.f (o.E
// of o.F, (*o).M->D of o.D), its Go type, and the rest of the path after it, for a target whose
// pointer is saved (stmt.go: saveOperands).
func (fg *funcGen) embeddedPtr(x *ast.SelectorExpr) (cppir.Expr, types.Type, []int) {
	sel := fg.fieldSel(x)
	t, index := sel.Recv(), sel.Index()
	last := 0 // the last step that goes through a pointer, after the first one
	for k, i := range index {
		if pt, ok := under(t).(*types.Pointer); ok {
			if k > 0 {
				last = k
			}
			t = pt.Elem()
		}
		t = under(t).(*types.Struct).Field(i).Type()
	}
	p, pt := fg.fieldSteps(fg.expr(x.X), sel.Recv(), index[:last], x.Pos())
	return p, pt, index[last:]
}

// fieldEffect returns the effect of reading the field x.f (order.go): through a pointer (also an
// embedded one), it reads memory that a call can change, and panics for nil. Reading a field of
// a value is reading the variable it is in, which has its own item.
func (fg *funcGen) fieldEffect(x *ast.SelectorExpr) effect {
	if sel := fg.fieldSel(x); sel != nil && sel.Indirect() {
		return effect{read: true, panics: true}
	}
	return effect{}
}

// fieldOwner returns the owner of the field x.f for its address (arrays.go: ownerOf): the
// struct x, whose owner it shares (inner), or the pointer p of p.f (ptr). For a field through
// an embedded pointer (o.D of an embedded *Deep), the owner is that pointer, which is not an
// expression of the source: embedded, and fieldPtr makes the address.
func (fg *funcGen) fieldOwner(x *ast.SelectorExpr) (inner, ptr ast.Expr, embedded bool) {
	sel := fg.fieldSel(x)
	switch {
	case sel == nil:
		return nil, nil, false
	case laterPointer(sel):
		return nil, nil, true
	case firstPointer(sel):
		return nil, x.X, false
	}
	return x.X, nil, false
}

// fieldPtr returns the C++ of &x.f one step at a time (arrays.go: elemPtr), for an owner that
// can't be read twice, gp::field_ptr(p, &T::f), with one call for each field from the last
// pointer on: p is the pointer x of p.f, the address of the struct x (no pointer on the way),
// or an embedded pointer on the way (o.D of an embedded *Deep: gp::field_ptr(o.Deep, &Deep::D)).
func (fg *funcGen) fieldPtr(x *ast.SelectorExpr) cppir.Expr {
	sel := fg.fieldSel(x)
	if sel == nil {
		fg.g.errorf(x.Pos(), "internal error: the address of %s, which is not a field", types.ExprString(x))
		return placeholder
	}
	t, index := sel.Recv(), sel.Index()
	last := -1 // the last step that goes through a pointer
	for k, i := range index {
		if pt, ok := under(t).(*types.Pointer); ok {
			last, t = k, pt.Elem()
		}
		t = under(t).(*types.Struct).Field(i).Type()
	}
	var p cppir.Expr
	t = sel.Recv()
	switch last {
	case -1:
		p = fg.elemPtr(x.X)
	case 0:
		p = fg.expr(x.X)
	default:
		p, t = fg.fieldSteps(fg.expr(x.X), t, index[:last], x.Pos()) // the embedded pointer
	}
	for _, i := range index[max(last, 0):] {
		if pt, ok := under(t).(*types.Pointer); ok {
			t = pt.Elem()
		}
		member := &cppir.Unary{Op: "&", X: cxxName(cppir.TypeString(fg.g.cxxType(t, x.Pos())) + "::" + fg.g.fieldName(t, i, x.Pos()))}
		p = call("gp::field_ptr", p, member)
		t = under(t).(*types.Struct).Field(i).Type()
	}
	return p
}

// sharedCopy returns the C++ x of the value e of an assignment to a variable that exists, or a
// copy of it, T(x), when the store could free e while the generated operator= reads it: e is a
// struct or an array that holds owners (pointers, slices, strings, ...) and is reached through a
// pointer or a slice (n = n.kids[0], *p = *p.next, s[0] = *s[0].next), which may be owned by the
// target, so that storing its first field drops the last owner of e. The copy is made before the
// store, and moved into the target (gp::slice's and gp::ptr's operator= copy first themselves).
func (fg *funcGen) sharedCopy(e ast.Expr, x cppir.Expr) cppir.Expr {
	t := fg.g.info.TypeOf(e)
	if _, ok := fg.temps[ast.Unparen(e)]; ok || t == nil || !(isStruct(t) || isArray(t)) || !mayOwn(t) || !fg.indirect(e) {
		return x
	}
	return construct(fg.g.cxxType(t, e.Pos()), x)
}

// indirect reports whether the addressable expression e is reached through a pointer or a slice
// (sharedCopy): *p, p.f, s[i], p[i], or a part of one of them.
func (fg *funcGen) indirect(e ast.Expr) bool {
	switch x := ast.Unparen(e).(type) {
	case *ast.StarExpr:
		return true
	case *ast.SelectorExpr:
		if sel := fg.fieldSel(x); sel != nil {
			return sel.Indirect() || fg.indirect(x.X)
		}
	case *ast.IndexExpr:
		if isArray(fg.g.info.TypeOf(x.X)) {
			return fg.indirect(x.X)
		}
		return isSlice(fg.g.info.TypeOf(x.X)) || arrayPtr(fg.g.info.TypeOf(x.X)) != nil
	}
	return false
}

// structConversion returns the C++ of the conversion T(x) to or from a struct type: x itself
// when both have the same C++ type (an alias); the others (between a named struct type and an
// identical anonymous one, or two named ones) come with step 3.9's conversions between types
// that are not identical.
func (fg *funcGen) structConversion(c *ast.CallExpr, to, from types.Type) cppir.Expr {
	arg := c.Args[0]
	if cppir.TypeString(fg.g.cxxType(to, c.Pos())) == cppir.TypeString(fg.g.cxxType(from, arg.Pos())) {
		return fg.expr(arg)
	}
	qual := types.RelativeTo(fg.g.pkg.Types)
	fg.g.unsupported(c.Pos(), "the conversion from "+types.TypeString(from, qual)+" to "+types.TypeString(to, qual)+" (Phase 03 step 3.9)")
	return placeholder
}

// goSizes are Go's own sizes (gc on amd64), for gc's rule of which values it copies into a
// temporary where they are converted to an interface (copiedStruct).
var goSizes = types.SizesFor("gc", "amd64")

// copiedStruct reports whether gc copies a value of the struct type t that is not addressable
// into a temporary in its place among the calls, where it is converted to an interface (an
// argument of fmt.Println: order.go, copied): unless the runtime converts it by value
// (walk/convert.go: dataWordFuncName), when it is 2 bytes, 4 or 8 bytes without pointers (in
// Go's sizes, not gocpp's), or its only part is a string or a slice.
func copiedStruct(t types.Type) bool {
	size, align, ptrs := goSizes.Sizeof(t), goSizes.Alignof(t), goHasPointers(t)
	switch {
	case size == 2 && align == 2, size == 4 && align == 4 && !ptrs, size == 8 && align == 8 && !ptrs:
		return false
	}
	if c := soleComponent(t); c != nil && (isString(c) || isSlice(c)) {
		return false
	}
	return true
}

// goHasPointers reports whether a value of type t holds a pointer in Go's layout (gc's
// HasPointers): a string, pointer, slice, map, chan, func or interface, or an array or a struct
// with one.
func goHasPointers(t types.Type) bool {
	switch u := under(t).(type) {
	case *types.Basic:
		return u.Info()&types.IsString != 0 || u.Kind() == types.UnsafePointer
	case *types.Array:
		return u.Len() > 0 && goHasPointers(u.Elem())
	case *types.Struct:
		for i := range u.NumFields() {
			if goHasPointers(u.Field(i).Type()) {
				return true
			}
		}
		return false
	}
	return true
}

// soleComponent returns the only part of t if t is a struct with one field or an array of one
// element (recursively), or t itself if it is neither (gc's SoleComponent); nil for more parts.
func soleComponent(t types.Type) types.Type {
	switch u := under(t).(type) {
	case *types.Struct:
		if u.NumFields() != 1 {
			return nil
		}
		return soleComponent(u.Field(0).Type())
	case *types.Array:
		if u.Len() != 1 {
			return nil
		}
		return soleComponent(u.Elem())
	}
	return t
}

// checkConversion reports the implicit conversion at pos of a value of type from to the type to
// if both are struct types that are not identical (implicitConversions).
func (g *gen) checkConversion(pos token.Pos, from, to types.Type) {
	if from == nil || to == nil || !isStruct(from) || !isStruct(to) || types.Identical(from, to) {
		return
	}
	qual := types.RelativeTo(g.pkg.Types)
	g.unsupported(pos, "the implicit conversion from "+types.TypeString(from, qual)+" to "+
		types.TypeString(to, qual)+" (Phase 03 step 3.9)")
}

// implicitConversions reports "not supported yet" for the implicit conversions in n between
// struct types that are not identical: a value of an anonymous struct type where the named struct
// type with the same fields is expected, or the other way round (var p Point = struct{ X, Y int
// }{1, 2}, f(struct{...}{...}), a return, an element of a composite literal, a map key, a range
// target with =, a case value of a switch). Go
// allows them because one of the types is unnamed; they are two C++ types, and step 3.9 adds the
// conversions where the types are not identical. sig is the signature of the function n is in
// (nil for the initializers of package variables).
func (g *gen) implicitConversions(n ast.Node, sig *types.Signature) {
	check := func(e ast.Expr, to types.Type) { g.checkConversion(e.Pos(), g.info.TypeOf(e), to) }
	// values checks the values of a list against the types: one value with several results
	// (a, b = f()), or one value for each type.
	values := func(list []ast.Expr, typeOf func(i int) types.Type, n int) {
		if len(list) == 1 && n > 1 {
			if tuple, ok := g.info.TypeOf(list[0]).(*types.Tuple); ok {
				for i := range min(tuple.Len(), n) {
					g.checkConversion(list[0].Pos(), tuple.At(i).Type(), typeOf(i))
				}
			}
			return
		}
		for i, e := range list {
			if i < n {
				check(e, typeOf(i))
			}
		}
	}
	sigs := []*types.Signature{sig}
	var stack []ast.Node
	ast.Inspect(n, func(n ast.Node) bool {
		if n == nil {
			if _, ok := stack[len(stack)-1].(*ast.FuncLit); ok {
				sigs = sigs[:len(sigs)-1]
			}
			stack = stack[:len(stack)-1]
			return true
		}
		stack = append(stack, n)
		switch x := n.(type) {
		case *ast.FuncLit:
			s, _ := g.info.TypeOf(x).(*types.Signature)
			sigs = append(sigs, s)
		case *ast.AssignStmt:
			if x.Tok == token.ASSIGN || x.Tok == token.DEFINE {
				values(x.Rhs, func(i int) types.Type {
					if isBlank(x.Lhs[i]) {
						return nil
					}
					return g.info.TypeOf(x.Lhs[i])
				}, len(x.Lhs))
			}
		case *ast.ValueSpec:
			if x.Type != nil {
				to := g.info.TypeOf(x.Type)
				values(x.Values, func(int) types.Type { return to }, len(x.Names))
			}
		case *ast.ReturnStmt:
			if s := sigs[len(sigs)-1]; s != nil {
				values(x.Results, func(i int) types.Type { return s.Results().At(i).Type() }, s.Results().Len())
			}
		case *ast.CallExpr:
			if g.info.Types[x.Fun].IsType() {
				return true // a conversion
			}
			s, ok := under(g.info.TypeOf(x.Fun)).(*types.Signature)
			if !ok {
				return true
			}
			params := s.Params()
			values(x.Args, func(i int) types.Type {
				if s.Variadic() && i >= params.Len()-1 {
					if x.Ellipsis.IsValid() {
						return params.At(params.Len() - 1).Type()
					}
					return params.At(params.Len() - 1).Type().(*types.Slice).Elem()
				}
				return params.At(i).Type()
			}, max(len(x.Args), params.Len()))
		case *ast.CompositeLit:
			t := g.litType(x)
			for i, elt := range x.Elts {
				kv, keyed := elt.(*ast.KeyValueExpr)
				switch u := under(t).(type) {
				case *types.Struct:
					if keyed {
						if k := fieldIndex(u, kv.Key.(*ast.Ident).Name); k >= 0 {
							check(kv.Value, u.Field(k).Type())
						}
					} else if i < u.NumFields() {
						check(elt, u.Field(i).Type())
					}
				case *types.Map:
					if keyed {
						check(kv.Key, u.Key())
						check(kv.Value, u.Elem())
					}
				case *types.Slice:
					if keyed {
						elt = kv.Value
					}
					check(elt, u.Elem())
				case *types.Array:
					if keyed {
						elt = kv.Value
					}
					check(elt, u.Elem())
				}
			}
		case *ast.IndexExpr:
			if m, ok := under(g.info.TypeOf(x.X)).(*types.Map); ok {
				check(x.Index, m.Key())
			}
		case *ast.BinaryExpr:
			if x.Op == token.EQL || x.Op == token.NEQ {
				check(x.Y, g.info.TypeOf(x.X))
			}
		case *ast.SwitchStmt:
			// The tag is compared with each case value, as by ==.
			if x.Tag != nil {
				for _, c := range x.Body.List {
					for _, e := range c.(*ast.CaseClause).List {
						check(e, g.info.TypeOf(x.Tag))
					}
				}
			}
		case *ast.RangeStmt:
			// for k, v = range x assigns the key and the value to targets that exist.
			if x.Tok != token.ASSIGN {
				break
			}
			var key, value types.Type
			switch u := under(g.info.TypeOf(x.X)).(type) {
			case *types.Slice:
				value = u.Elem()
			case *types.Array:
				value = u.Elem()
			case *types.Pointer:
				if a, ok := under(u.Elem()).(*types.Array); ok {
					value = a.Elem()
				}
			case *types.Map:
				key, value = u.Key(), u.Elem()
			}
			for _, t := range []struct {
				e    ast.Expr
				from types.Type
			}{{x.Key, key}, {x.Value, value}} {
				if t.e != nil && !isBlank(t.e) {
					g.checkConversion(t.e.Pos(), t.from, g.info.TypeOf(t.e))
				}
			}
		}
		return true
	})
}
