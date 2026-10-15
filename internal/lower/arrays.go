package lower

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"strconv"

	"github.com/huuhait/gocpp/internal/cppir"
)

// Arrays (Phase 03 README §2.3, step 3.6; the runtime is gocpp/array.hpp):
//
//	Go                          C++
//	[N]T                        gp::array<T, N>
//	var a [3]T                  gp::array<T, 3> a{};  (all zero)
//	[3]T{a, b}, [...]T{a, b}    gp::array<T, 3>{a, b} (the missing elements are zero)
//	[5]T{2: a, b}               gp::array_keyed<T, 5>({{2, a}, {3, b}}) (keyed: the rest zero)
//	[2][2]T{{a, b}, {c, d}}     gp::array<gp::array<T, 2>, 2>{gp::array<T, 2>{a, b}, ...}
//	a[i], p[i] (p is *[N]T)     a[i], (*p)[i]: a T&, bounds checked; a nil p panics first
//	len(a), cap(a), len(p)      N (go/types folds it), or gp::len(f()) when it has calls
//	a == b, a != b              a == b, a != b (when T is comparable)
//	a[lo:hi], a[lo:hi:max]      gp::sub(&a, lo, hi), gp::sub(&a, lo, hi, max), &a as below
//	p[lo:hi] (p is *[N]T)       gp::sub(p, lo, hi)
//	&a[i]                       a pointer that shares the owner of a (addrOf)
//	[N]T(s), (*[N]T)(s)         gp::to_array<N>(s), gp::to_array_ptr<N>(s)
//	for i := range a            for (int64_t gp_i0 = 0; gp_i0 < N; gp_i0++) (control.go)
//	for i, v := range a         for (auto gp_it0 = gp::range_array_copy(a); gp::range_next(gp_it0);)
//	                            (rangeArray: or gp::range_array(a), without the copy)
//
// An array is a value: assigning, passing and returning one copies it, and gp::array is a C++
// class type (a call of a user function with an array argument is qualified, funcCallName, and
// a temporary is moved into its variable). The address of an array, or of a part of one, needs
// the array to live in a heap block or a package variable: the boxing analysis (box.go) boxes
// each local array variable whose address is taken or that is sliced, and addrOf builds the
// pointer from the owner table (addr.go).
//
// Go's order of evaluation (order.go): an element of an array is part of the array's variable,
// so a[i] is read where the variable is read: after the calls of its scope for a package or a
// boxed variable (which a call can change) and for an array behind a pointer (p[i], (*p)[i]),
// and anywhere for a plain local, which no call can change. a[i] with an index that is not a
// constant can panic, so it is after the calls too. [N]T(s) reads the elements of s, and both
// conversions panic for a short s, after the calls of their scope, as gc does.
//
// Arrays larger than 128 KiB: a local variable of such a type is boxed (box.go) and built in its
// box from a literal or a conversion (largeBox), a parameter is a const reference copied into a
// box at the function's entry (decl.go: prepare), and a range copies it to the heap
// (range_array_copy). The other temporaries of such a type, results of functions, and literals
// and conversions that don't initialize a new variable, are on the C++ stack, which doesn't grow
// like Go's: divergence D24 (docs/spec/language.md).

// arrayType returns gp::array<elem, n>, the C++ type of an array of n elem.
func arrayType(elem cppir.Type, n int64) cppir.Type {
	return &cppir.TypeName{Name: "gp::array", Args: []cppir.Node{elem, lit(strconv.FormatInt(n, 10))}}
}

// isArray reports whether t is an array type (through names and aliases).
func isArray(t types.Type) bool {
	_, ok := under(t).(*types.Array)
	return ok
}

// arrayPtr returns the array type that the pointer type t points to, or nil if t is not a
// pointer to an array.
func arrayPtr(t types.Type) *types.Array {
	p, ok := under(t).(*types.Pointer)
	if !ok {
		return nil
	}
	a, _ := under(p.Elem()).(*types.Array)
	return a
}

// arrayOf returns the array type of t, an array or a pointer to an array, or nil.
func arrayOf(t types.Type) *types.Array {
	if a, ok := under(t).(*types.Array); ok {
		return a
	}
	return arrayPtr(t)
}

// arrayLit returns the C++ of an array literal: gp::array<T, N>{a, b}, whose braced list C++
// evaluates in order, each element converted to T (a constant is written in T's type by
// go/types' record of it), and the missing ones zero. A keyed literal gives each element its
// index (a key, or the index after the element before it): gp::array_keyed<T, N>({{2, a}, {3,
// b}}), which evaluates the elements in source order too. An element that is a literal of its
// own names its type (gp::array<T, 2>{...}, never a bare {}), because C++ would take a bare {}
// as the initializer of gp::array's member.
func (fg *funcGen) arrayLit(e *ast.CompositeLit) cppir.Expr {
	return fg.g.arrayLitValue(e, fg.expr)
}

// arrayLitValue returns arrayLitOf(e, elem) as a value of e's type: a literal of a named array
// type is the gp::array converted to it (step 3.9), Grid(gp::array<int64_t, 3>{1, 2, 3}).
func (g *gen) arrayLitValue(e *ast.CompositeLit, elem func(ast.Expr) cppir.Expr) cppir.Expr {
	x := g.arrayLitOf(e, elem)
	if x == nil {
		return nil
	}
	return g.wrapNamed(g.litType(e), x, e.Lbrace)
}

// arrayLitOf returns the C++ of the array literal e (arrayLit), with elem's C++ for each
// element, or nil if elem returns nil for one: a gp::array, also for a named array type.
func (g *gen) arrayLitOf(e *ast.CompositeLit, elem func(ast.Expr) cppir.Expr) cppir.Expr {
	t := under(g.litType(e)) // [N]T of an elided &[N]T{...} too, and of a named array type
	at := t.(*types.Array)
	var elems, pairs []cppir.Expr
	var index int64
	for _, elt := range e.Elts {
		if kv, ok := elt.(*ast.KeyValueExpr); ok {
			index, _ = constant.Int64Val(constant.ToInt(g.info.Types[kv.Key].Value))
			elt = kv.Value
		}
		x := elem(elt)
		if x == nil {
			return nil
		}
		elems = append(elems, x)
		pairs = append(pairs, &cppir.InitList{Elems: []cppir.Expr{lit(strconv.FormatInt(index, 10)), x}})
		index++
	}
	if !hasKeys(e) {
		return &cppir.InitList{Type: g.cxxType(t, e.Lbrace), Elems: elems}
	}
	targs := []cppir.Node{g.cxxType(at.Elem(), e.Lbrace), lit(strconv.FormatInt(at.Len(), 10))}
	return templateCall("gp::array_keyed", targs, &cppir.InitList{Elems: pairs})
}

// largeBox returns the C++ that makes the box of a new variable of the C++ type typ that is
// larger than 128 KiB (box.go: boxLarge), or the block of &[N]T{...} of such a type (addr.go),
// with the value: an array literal or [N]T(s) is built in the new block itself, gp::new_<A>()
// for an empty literal, gp::new_array<T, N>({a, b}), gp::new_array_keyed<T, N>({{2, a}}) or
// gp::new_to_array<N>(s) (array.hpp), and a struct literal is gp::new_init<T>(...) (newStruct,
// structs.go), because gp::new_<A>(A{a, b}) would first build the value in a temporary on the
// C++ stack, which could overflow it (D24). Any other value is gp::new_<A>(value).
func largeBox(typ cppir.Type, value cppir.Expr) cppir.Expr {
	if x := largeNamedBox(typ, value); x != nil {
		return x // a literal of a named array type (step 3.9, named.go)
	}
	switch x := value.(type) {
	case *cppir.InitList:
		if x.Type != nil && len(x.Elems) == 0 {
			return newBox(typ, nil) // the zero value (also of a struct): gp::new_<A>()
		}
		if tn, ok := x.Type.(*cppir.TypeName); ok && tn.Name == "gp::array" {
			return templateCall("gp::new_array", tn.Args, &cppir.InitList{Elems: x.Elems})
		}
		if x.Type != nil {
			return newStruct(typ, x) // a struct literal (structs.go)
		}
	case *cppir.Call:
		if n, ok := x.Fun.(*cppir.Name); ok {
			switch n.Name {
			case "gp::array_keyed", "gp::to_array":
				return &cppir.Call{Fun: &cppir.Name{Name: "gp::new_" + n.Name[len("gp::"):], Args: n.Args}, Args: x.Args}
			}
		}
	}
	return newBox(typ, value)
}

// sameTypeInit returns the initializer of a new variable of the C++ type typ whose value is
// value: an array literal of that type, gp::array<T, N>{a, b}, is its braced list, x{a, b},
// which doesn't name the type twice.
func sameTypeInit(typ cppir.Type, value cppir.Expr) cppir.Expr {
	if l, ok := value.(*cppir.InitList); ok && l.Type != nil &&
		cppir.TypeString(l.Type) == cppir.TypeString(typ) {
		return &cppir.InitList{Elems: l.Elems}
	}
	return value
}

// hasKeys reports whether the composite literal e has a key: value element.
func hasKeys(e *ast.CompositeLit) bool {
	for _, elt := range e.Elts {
		if _, ok := elt.(*ast.KeyValueExpr); ok {
			return true
		}
	}
	return false
}

// arrayIndex returns the C++ of a[i] for an array a, a T& that can be assigned (gp::array's
// operator[], which checks the bounds), or of p[i] for a pointer p to an array, (*p)[i]: a nil
// p panics with the nil dereference before the index is checked, as in Go.
func (fg *funcGen) arrayIndex(e *ast.IndexExpr) cppir.Expr {
	x := fg.expr(e.X)
	if arrayPtr(fg.g.info.TypeOf(e.X)) != nil {
		x = deref(x)
	}
	return &cppir.Index{X: x, Index: fg.intExpr(e.Index)}
}

// arraySlice returns the C++ of a slice expression on an addressable array, a[lo:hi] (the
// slice shares the array: gp::sub of a pointer to it, addrOf), or on a pointer p to an array,
// p[lo:hi], gp::sub(p, lo, hi). The bounds are checked against N (Go's "with length N"), after
// a nil p's nil dereference panic. A missing low bound is 0; a[:] is gp::sub(&a, 0).
func (fg *funcGen) arraySlice(e *ast.SliceExpr) cppir.Expr {
	var p cppir.Expr
	if xt := fg.g.info.TypeOf(e.X); arrayPtr(xt) != nil {
		p = arrayPtrValue(xt, fg.expr(e.X))
	} else {
		p = fg.addrOf(e.X)
		if isNewtype(xt) {
			p = call("gp::base_ptr", p) // a named array type (step 3.9): a gp::ptr<Buf>
		}
	}
	var lo cppir.Expr = lit("0")
	if e.Low != nil {
		lo = fg.intExpr(e.Low)
	}
	switch {
	case e.High == nil:
		return call("gp::sub", p, lo)
	case e.Max != nil:
		return call("gp::sub", p, lo, fg.intExpr(e.High), fg.intExpr(e.Max))
	}
	return call("gp::sub", p, lo, fg.intExpr(e.High))
}

// The kinds of owners of an addressable expression inside an array or a struct (ownerOf).
const (
	ownerNone     = iota // not addressable, or a form that comes later
	ownerPkg             // a package variable: the static owner
	ownerBox             // a boxed local variable: its box
	ownerPtr             // *p, p[i] of a pointer p to an array, p.f of a pointer p to a struct: p
	ownerSlice           // s[i] of a slice s: the slice's array
	ownerEmbedded        // a field through an embedded pointer (structs.go): that pointer
)

// ownerOf returns the owner of the addressable expression e (Phase 03 README §2.2, the owner
// table): e is a variable, or an element of an array or a field of a struct, maybe nested
// (a[i][j], s[i].f.g), that is in the variable, behind a pointer or in a slice's array. It
// returns the variable, the pointer or the slice expression, and its kind.
func (fg *funcGen) ownerOf(e ast.Expr) (ast.Expr, int) {
	for {
		switch x := ast.Unparen(e).(type) {
		case *ast.Ident:
			v, ok := fg.g.info.Uses[x].(*types.Var)
			switch {
			case !ok:
			case isPkgVar(v):
				return x, ownerPkg
			case fg.isBoxed(v):
				return x, ownerBox
			}
			return nil, ownerNone
		case *ast.StarExpr:
			return x.X, ownerPtr
		case *ast.IndexExpr:
			xt := fg.g.info.TypeOf(x.X)
			switch {
			case isArray(xt):
				e = x.X
				continue
			case arrayPtr(xt) != nil:
				return x.X, ownerPtr
			case isSlice(xt):
				return x.X, ownerSlice
			}
		case *ast.SelectorExpr:
			// A field (structs.go): of the struct x, which has the owner, or through the
			// pointer p of p.f, or through an embedded pointer.
			inner, p, embedded := fg.fieldOwner(x)
			switch {
			case inner != nil:
				e = inner
				continue
			case p != nil:
				return p, ownerPtr
			case embedded:
				return x, ownerEmbedded
			}
		}
		return nil, ownerNone
	}
}

// addrOf returns the C++ of &e, a gp::ptr to the addressable expression e (a variable, or an
// element of an array inside one, behind a pointer or in a slice's array) that shares the owner
// of the block that e is in (the owner table, addr.go):
//
//	&garr, &garr[i][j] (package)   gp::static_ptr(garr), gp::static_ptr(garr[i][j])
//	&a, &a[i] (a is boxed)         a, gp::ptr<T>(a, &(*a)[i])
//	&p[i], &(*p)[i] (p a variable) gp::ptr<T>(p, &(*p)[i])
//	&s[j][i] (s a variable)        gp::ptr<T>(s.gp_owner(), &s[j][i])
//	&f()[i], &f()[j][i]            gp::elem_ptr(f(), i), gp::elem_ptr(gp::elem_ptr(f(), j), i)
//
// The aliasing form reads the owner twice, so it is only for an owner that reads the same twice
// (stableOwner: a variable, *p, or an index of one). Any other owner (a call, a conversion) is
// built one step at a time with gp::elem_ptr, which evaluates each operand once. *p as a whole
// (to slice it, (*p)[:]) is p.
func (fg *funcGen) addrOf(e ast.Expr) cppir.Expr {
	owner, kind := fg.ownerOf(e)
	typ := ptrType(fg.typeOfExpr(e))
	switch kind {
	case ownerPkg:
		return call("gp::static_ptr", fg.expr(e))
	case ownerBox:
		box := cxxName(fg.cname(fg.g.info.Uses[owner.(*ast.Ident)], owner.Pos()))
		if ast.Unparen(e) == owner {
			return box
		}
		return construct(typ, box, &cppir.Unary{Op: "&", X: fg.expr(e)})
	case ownerPtr, ownerSlice:
		if s, ok := ast.Unparen(e).(*ast.StarExpr); ok {
			return fg.expr(s.X)
		}
		if !fg.stableOwner(owner) {
			return fg.elemPtr(e)
		}
		o := fg.expr(owner)
		if kind == ownerSlice {
			o = &cppir.Call{Fun: &cppir.Member{X: o, Name: "gp_owner"}}
		}
		return construct(typ, o, &cppir.Unary{Op: "&", X: fg.expr(e)})
	case ownerEmbedded:
		return fg.elemPtr(e) // gp::field_ptr from the embedded pointer (structs.go: fieldPtr)
	}
	fg.g.errorf(e.Pos(), "internal error: the address of %s, which has no owner", types.ExprString(e))
	return placeholder
}

// elemPtr returns the C++ of &e for an element e of an array or a slice whose owner is not
// stable (addrOf), one step at a time: gp::elem_ptr(s, i) for s[i] of a slice, gp::elem_ptr(p,
// i) for p[i] or (*p)[i] of a pointer to an array, and gp::elem_ptr(&a, i) for a[i] of an
// array a that is itself such an element.
func (fg *funcGen) elemPtr(e ast.Expr) cppir.Expr {
	switch x := ast.Unparen(e).(type) {
	case *ast.StarExpr:
		return fg.expr(x.X)
	case *ast.IndexExpr:
		xt := fg.g.info.TypeOf(x.X)
		var base cppir.Expr
		switch {
		case isArray(xt):
			base = fg.elemPtr(x.X)
			if isNewtype(xt) {
				base = call("gp::base_ptr", base) // a named array type (step 3.9)
			}
		default: // a slice, or a pointer to an array
			base = arrayPtrValue(xt, fg.expr(x.X))
		}
		return call("gp::elem_ptr", base, fg.intExpr(x.Index))
	case *ast.SelectorExpr:
		return fg.fieldPtr(x) // structs.go
	}
	fg.g.errorf(e.Pos(), "internal error: the address of %s, which has no owner", types.ExprString(e))
	return placeholder
}

// stableOwner reports whether the owner e of an address (addrOf) gives the same value when it
// is read twice in one expression: a variable, a temporary of the order (order.go), *p and an
// element v[i] of such an e with an index that is a constant or such an e too. Nothing between
// the two reads can change them: the calls of the statement ran before (order.go).
func (fg *funcGen) stableOwner(e ast.Expr) bool {
	e = ast.Unparen(e)
	if _, ok := fg.temps[e]; ok || fg.constValue(e) != nil {
		return true
	}
	switch x := e.(type) {
	case *ast.Ident:
		_, ok := fg.g.info.Uses[x].(*types.Var)
		return ok
	case *ast.StarExpr:
		return fg.stableOwner(x.X)
	case *ast.IndexExpr:
		return fg.stableOwner(x.X) && fg.stableOwner(x.Index)
	case *ast.SelectorExpr:
		return fg.fieldSel(x) != nil && fg.stableOwner(x.X) // a field: x.p, p.q
	}
	return false
}

// arrayConversion returns the C++ of a conversion with arrays, or nil if it is none:
// [N]T(s) of a slice s is gp::to_array<N>(s), a copy of s's first N elements, (*[N]T)(s) is
// gp::to_array_ptr<N>(s), a pointer that shares s's array (nil for a nil s when N is 0, as
// in Go), both with Go's panic when len(s) < N; and [N]T(a) of an array of the same C++ type is
// a itself.
func (fg *funcGen) arrayConversion(c *ast.CallExpr, to, from types.Type, x cppir.Expr) cppir.Expr {
	arg := c.Args[0]
	switch {
	case isArray(to) && isSlice(from):
		n := under(to).(*types.Array).Len()
		return templateCall("gp::to_array", []cppir.Node{lit(strconv.FormatInt(n, 10))}, x)
	case arrayPtr(to) != nil && isSlice(from):
		n := arrayPtr(to).Len()
		p := templateCall("gp::to_array_ptr", []cppir.Node{lit(strconv.FormatInt(n, 10))}, x)
		if elem := under(to).(*types.Pointer).Elem(); isNewtype(elem) {
			// (*Buf)(s) for type Buf [8]byte: a pointer to the gp::array as one to the Buf
			// (step 3.9), which is that array (its base class).
			return templateCall("gp::ptr_cast", []cppir.Node{fg.g.cxxType(elem, c.Pos())}, p)
		}
		return p
	case isArray(to) && isArray(from):
		if cppir.TypeString(fg.g.cxxType(to, c.Pos())) == cppir.TypeString(fg.g.cxxType(from, arg.Pos())) {
			return x
		}
	}
	return nil
}

// conversionEffect returns the effect of the conversion c of a slice to an array or to a
// pointer to an array (order.go): [N]T(s) reads the elements of s, which a call can change, and
// both panic when s is shorter than N (never for N == 0), after the calls of their scope, as gc
// does.
func (fg *funcGen) conversionEffect(c *ast.CallExpr) effect {
	if len(c.Args) != 1 || !isSlice(fg.g.info.TypeOf(c.Args[0])) {
		return effect{}
	}
	to := fg.g.info.TypeOf(c)
	if a := arrayOf(to); a != nil {
		return effect{read: isArray(to), panics: a.Len() > 0}
	}
	return effect{}
}

// indexEffect returns the effect of a[i] or p[i] of an array or a pointer to an array
// (order.go): p[i] reads through p (memory that a call can change) and panics for a nil p; an
// index that is not a constant can panic. Reading a[i] itself is reading the variable a (or the
// slice element, ...) that it is in, which has its own item.
func (fg *funcGen) indexEffect(x *ast.IndexExpr) effect {
	if arrayPtr(fg.g.info.TypeOf(x.X)) != nil {
		return effect{read: true, panics: true}
	}
	return effect{panics: fg.constValue(x.Index) == nil}
}

// addressable reports whether e is an addressable array or struct expression (not a call, a
// literal or a conversion): gc copies a value that is not addressable into a temporary where it is converted
// to an interface (order.go: copied).
func (fg *funcGen) addressable(e ast.Expr) bool {
	switch x := ast.Unparen(e).(type) {
	case *ast.Ident:
		_, ok := fg.g.info.Uses[x].(*types.Var)
		return ok
	case *ast.StarExpr:
		return true
	case *ast.IndexExpr:
		xt := fg.g.info.TypeOf(x.X)
		return isSlice(xt) || arrayPtr(xt) != nil || isArray(xt) && fg.addressable(x.X)
	case *ast.SelectorExpr:
		// A field through a pointer, or of an addressable struct.
		sel := fg.fieldSel(x)
		return sel != nil && (sel.Indirect() || fg.addressable(x.X))
	}
	return false
}

// saveArrayElem returns the C++ of the target e of an assignment, an element of an array (a[i],
// p[i], (*p)[i], a[i][j], s[j][i]), with the operands that an assignment before it can change
// in temporaries (saveOperands, stmt.go; save returns a temporary's name, or nil if the operand
// needs none): the index, the pointer p and the slice s, but never the array variable a, whose
// place doesn't change (a, a[0] = b, 5 stores 5 into the new a, as in Go). It returns nil if
// nothing needs a temporary.
func (fg *funcGen) saveArrayElem(e ast.Expr, save func(ast.Expr) cppir.Expr) cppir.Expr {
	switch x := ast.Unparen(e).(type) {
	case *ast.StarExpr:
		if p := save(x.X); p != nil {
			return deref(p)
		}
	case *ast.IndexExpr:
		xt := fg.g.info.TypeOf(x.X)
		var base cppir.Expr
		switch {
		case isArray(xt):
			base = fg.saveArrayElem(x.X, save)
		case arrayPtr(xt) != nil:
			if p := save(x.X); p != nil {
				base = deref(p)
			}
		case isSlice(xt):
			base = save(x.X)
		}
		idx := save(x.Index)
		if base == nil && idx == nil {
			return nil
		}
		if base == nil {
			base = fg.expr(x.X)
			if arrayPtr(xt) != nil {
				base = deref(base)
			}
		}
		if idx == nil {
			idx = fg.intExpr(x.Index)
		}
		return &cppir.Index{X: base, Index: idx}
	case *ast.SelectorExpr:
		return fg.saveField(x, save) // structs.go
	}
	return nil
}

// inArray reports whether the index expression x is an element of an array: a[i] or p[i].
func (fg *funcGen) inArray(x *ast.IndexExpr) bool {
	return arrayOf(fg.g.info.TypeOf(x.X)) != nil
}

// rangeArray lowers a range loop over an array or a pointer to an array with a value variable,
// for i, v := range x, and reports whether it did: the loops without one (for i := range x, for
// range x, for i, _ := range x) are counters to N (rangeStmt, control.go: rangeCount). The value
// is read at the start of each iteration (rangeIter, control.go), from:
//   - a copy of the array, gp::range_array_copy(x), when the body could change it (Go ranges
//     over a copy of an array: for _, v := range a { a[1] = 100 } still sees the old a[1]), and
//     for an array that the loop must hold itself: a value (f(), a literal), or one in a block
//     that only a temporary owns (*f(), f()[i], *(&[N]T{...})), which is freed when the loop's
//     init ends (stableArray);
//   - the array itself, gp::range_array(x), when the body can't change it (rangeCopies);
//   - the pointer, gp::range_array(p), for p of type *[N]T: Go doesn't copy the array then,
//     so a write in the body is seen, and a nil p panics at the first iteration (none for N ==
//     0).
func (fg *funcGen) rangeArray(s *ast.RangeStmt) bool {
	if s.Value == nil || isBlank(s.Value) {
		return false
	}
	start := "gp::range_array"
	if !isPointer(fg.g.info.TypeOf(s.X)) && fg.rangeCopies(s) {
		start = "gp::range_array_copy"
	}
	fg.rangeIter(s, start, func(it cppir.Expr) cppir.Expr { return &cppir.Call{Fun: &cppir.Member{X: it, Name: "gp_v"}} })
	return true
}

// rangeCount returns the number of iterations of a range over an array or a pointer to an array
// without a value variable, N, if the loop doesn't evaluate the array (the spec, "For statements
// with range clause"): when at most one iteration variable is present (for i := range x, not for
// i, _ := range x) and len(x) is a constant (x has no call). So for i := range *p of a nil p
// doesn't panic, and for i, _ := range *p does. It returns nil for the others, whose count is
// gp::len(x) (rangeLimit), which evaluates x, and for the other ranges.
func (fg *funcGen) rangeCount(s *ast.RangeStmt) constant.Value {
	a := arrayOf(fg.g.info.TypeOf(s.X))
	if a == nil || s.Value != nil || hasCall(fg.g.info, s.X) {
		return nil
	}
	return constant.MakeInt64(a.Len())
}

// rangeLimit returns the C++ of the number of iterations of a range over x whose count is not
// a constant: x for an integer, gp::len(x) for an array (or a pointer to one), which evaluates
// x, for its calls.
func (fg *funcGen) rangeLimit(x ast.Expr) cppir.Expr {
	if arrayOf(fg.g.info.TypeOf(x)) != nil {
		return call("gp::len", fg.expr(x))
	}
	return fg.expr(x)
}

// hasCall reports whether e has a function call (also of a builtin) whose value is not a
// constant, or a receive: then len(e) of an array is not a constant, and e is evaluated (the
// spec, "Length and capacity"; go/types' hasCallOrRecv). A conversion is no call.
func hasCall(info *types.Info, e ast.Expr) bool {
	found := false
	ast.Inspect(e, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.CallExpr:
			if !info.Types[x.Fun].IsType() && info.Types[x].Value == nil {
				found = true
			}
		case *ast.UnaryExpr:
			if x.Op == token.ARROW {
				found = true
			}
		case *ast.FuncLit:
			return false // its body runs only when it is called
		}
		return !found
	})
	return found
}

// rangeCopies reports whether `for i, v := range x` over the array x must range over a copy of
// x (rangeArray): x is not reached from a variable (stableArray: a call, a literal, *f(),
// f()[i], whose block nothing else keeps alive while the loop runs), or the body could change x. For a local variable that is not boxed (or an element of
// one), only an assignment to it or to a part of it in the body can (no call can reach it:
// box.go boxes every variable whose address is taken or that a closure changes). For any other
// array (a package or a boxed variable, *p, s[i]) the body could also change it with a call or a
// write to memory (through a pointer, a slice or a package variable), or free it, by a write to
// a variable that may hold the last pointer to its block (p = nil for x = *p), so any call or
// such write means a copy. The targets of for k, v = range x count too: a target that is x or
// a part of it (for i, x[0] = range x), and a call in one (for i, t[f()] = range x), since they
// are evaluated in every iteration.
func (fg *funcGen) rangeCopies(s *ast.RangeStmt) bool {
	if !fg.stableArray(s.X) {
		return true
	}
	b := &boxer{info: fg.g.info}
	// The local variable that x is or is in (a, a[i]), if it is not boxed.
	local := b.root(s.X)
	if local != nil && fg.isBoxed(local) {
		local = nil
	}
	// write reports whether storing to the target e could change x (or free it).
	write := func(e ast.Expr) bool {
		v := b.root(e)
		if local != nil {
			return v == local
		}
		return v == nil || fg.isBoxed(v) || mayOwn(v.Type())
	}
	if s.Tok == token.ASSIGN {
		for _, e := range []ast.Expr{s.Key, s.Value} {
			if e != nil && !isBlank(e) && write(e) {
				return true
			}
		}
	}
	changes := false
	inspect := func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.AssignStmt:
			for _, e := range x.Lhs {
				if x.Tok == token.DEFINE {
					if id, ok := e.(*ast.Ident); ok && fg.g.info.Defs[id] != nil {
						continue // a new variable
					}
				}
				if !isBlank(e) && write(e) {
					changes = true
				}
			}
		case *ast.IncDecStmt:
			changes = changes || write(x.X)
		case *ast.RangeStmt:
			if x.Tok == token.ASSIGN {
				for _, e := range []ast.Expr{x.Key, x.Value} {
					if e != nil && !isBlank(e) && write(e) {
						changes = true
					}
				}
			}
		case *ast.CallExpr:
			if local == nil && !fg.g.info.Types[x.Fun].IsType() && !fg.pureBuiltin(x) {
				changes = true
			}
		}
		return !changes
	}
	ast.Inspect(s.Body, inspect)
	if s.Tok == token.ASSIGN {
		// The targets of for k, v = range x are evaluated in every iteration: a call in them
		// (for i, t[f()] = range x) can change or free x too.
		for _, e := range []ast.Expr{s.Key, s.Value} {
			if e != nil && !changes {
				ast.Inspect(e, inspect)
			}
		}
	}
	return changes
}

// stableArray reports whether the array x is reached from a variable, so that a reference to it
// stays valid while a range loop runs (if the body doesn't change or free it, rangeCopies): a
// variable, an element of such an array (a[i]), or an array that a pointer or a slice that is
// such a variable points into: *p, p[i], s[i], (*pp)[i] or s[j][i] with p, pp and s variables.
// Not an array in a block that only a temporary owns: *f(), f()[i], *(&[N]T{...}), **f(), and not
// a map's entry m[k] or what it points into (m[k][i]): gp::get gives a copy (step 3.7).
func (fg *funcGen) stableArray(x ast.Expr) bool {
	switch e := ast.Unparen(x).(type) {
	case *ast.Ident:
		_, ok := fg.g.info.Uses[e].(*types.Var)
		return ok
	case *ast.StarExpr:
		return fg.varOwner(e.X)
	case *ast.IndexExpr:
		if isArray(fg.g.info.TypeOf(e.X)) {
			return fg.stableArray(e.X)
		}
		// A slice or a pointer to an array; not a map, whose m[k] is a copy (gp::get).
		return !isMap(fg.g.info.TypeOf(e.X)) && fg.varOwner(e.X)
	}
	return false
}

// varOwner reports whether the pointer or the slice e that holds an array's block is a variable
// or is read from one without a call: p, *pp, s[i], ss[i][j] (stableArray).
func (fg *funcGen) varOwner(e ast.Expr) bool {
	switch x := ast.Unparen(e).(type) {
	case *ast.Ident:
		_, ok := fg.g.info.Uses[x].(*types.Var)
		return ok
	case *ast.StarExpr:
		return fg.varOwner(x.X)
	case *ast.IndexExpr:
		// A map's m[k] is a copy of the entry (gp::get), a temporary: not a variable.
		return !isMap(fg.g.info.TypeOf(x.X)) && fg.varOwner(x.X) && !hasCall(fg.g.info, x.Index)
	}
	return false
}

// mayOwn reports whether a value of type t can hold a pointer to a heap block (an owner): a
// pointer, a slice, a map, a func, a channel or an interface, or an array or a struct with one.
// A string owns only its own bytes.
func mayOwn(t types.Type) bool {
	switch u := under(t).(type) {
	case *types.Basic:
		return false
	case *types.Array:
		return mayOwn(u.Elem())
	case *types.Struct:
		for i := range u.NumFields() {
			if mayOwn(u.Field(i).Type()) {
				return true
			}
		}
		return false
	}
	return true
}

// pureBuiltin reports whether c calls a builtin that writes no memory that a variable can see:
// len, cap, new, make, min, max, print, println, panic (rangeCopies).
func (fg *funcGen) pureBuiltin(c *ast.CallExpr) bool {
	for _, name := range []string{"len", "cap", "new", "make", "min", "max", "print", "println", "panic"} {
		if fg.isBuiltin(c, name) {
			return true
		}
	}
	return false
}

// typeOfExpr returns the C++ type of the expression e: of its Go type, or, for a variable of a
// type with C/C++ parts ([3]C.int), of its declaration (varType).
func (fg *funcGen) typeOfExpr(e ast.Expr) cppir.Type {
	t := fg.g.info.TypeOf(e)
	if id, ok := ast.Unparen(e).(*ast.Ident); ok && !complete(t) {
		if v, ok := fg.g.info.Uses[id].(*types.Var); ok {
			return fg.g.varType(v, id.Pos())
		}
	}
	return fg.g.cxxType(t, e.Pos())
}
