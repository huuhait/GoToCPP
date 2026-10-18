package lower

import (
	"go/ast"
	"go/token"
	"go/types"
	"strconv"
	"strings"

	"github.com/huuhait/gocpp/internal/cppir"
)

// Conversions between unsafe.Pointer and C/C++ pointers (Phase 04 README §2.7, step 4.10; the
// runtime is gp::cgo::as in gocpp/interop.hpp and gocpp/unsafe.hpp). cgo's rules are Go's: a C
// pointer converts to another pointer type only through unsafe.Pointer (a C void* is an
// unsafe.Pointer for cgo):
//
//	unsafe.Pointer(cp)             gp::cgo::as<gp::ptr<void>>(cp)    cp: a C pointer, a C function
//	                                                                 or an address; shares gocpp's
//	                                                                 foreign owner: owns nothing
//	(*C.char)(unsafe.Pointer(b))   gp::cgo::as<char*>(...)           a raw pointer
//	(*[16]C.char)(p)               gp::cgo::as<gp::ptr<gp::array<char, 16>>>(p)
//	                                                                 a gocpp pointer to an array of
//	                                                                 C elements, which can be sliced
//	(*byte)(cvoid)                 gp::cgo::as<gp::ptr<uint8_t>>(cvoid)   cvoid: a C void*
//
// A C pointer converted to a number needs unsafe.Pointer too (uintptr(unsafe.Pointer(cp))).

// cUnsafeConversion returns the C++ of the conversion c of a C/C++ value (gp_cgoh.As[T](x) after
// the check's rewrite) to unsafe.Pointer or to a pointer to a gocpp type: gp::cgo::as<T>(x).
func (fg *funcGen) cUnsafeConversion(c *ast.CallExpr, to types.Type) (cppir.Expr, bool) {
	if !isUnsafePointer(to) && !isPointer(to) {
		return nil, false
	}
	return templateCall("gp::cgo::as", []cppir.Node{fg.g.cxxType(to, c.Pos())}, fg.cValueOf(c.Args[0])), true
}

// cPointerType returns the type expression of the pointer type that the conversion c converts
// to, when that type has a C/C++ part ((*C.char)(p), (*[16]C.char)(p)), or nil: go/types has no
// complete type for it, so check couldn't rewrite the call (a conversion to a type that is
// valid is rewritten only for an opaque operand: cUnsafeConversion).
func (g *gen) cPointerType(c *ast.CallExpr) ast.Expr {
	if len(c.Args) != 1 {
		return nil
	}
	star, ok := ast.Unparen(c.Fun).(*ast.StarExpr)
	if !ok {
		return nil
	}
	if tv, ok := g.info.Types[c.Fun]; ok && tv.IsType() && complete(tv.Type) {
		return nil
	}
	return star
}

// cPointerConversion returns the C++ of the conversion c to a pointer type with a C/C++ part
// (cPointerType): gp::cgo::as<T>(x), where T is the raw pointer of a C/C++ type or the gocpp
// pointer of an array or another gocpp type with C/C++ parts (ctypes.go: typeExpr).
func (fg *funcGen) cPointerConversion(c *ast.CallExpr) (cppir.Expr, bool) {
	star := fg.g.cPointerType(c)
	if star == nil {
		return nil, false
	}
	return templateCall("gp::cgo::as", []cppir.Node{fg.g.typeExpr(star)}, fg.cValueOf(c.Args[0])), true
}

// unsafeCallOK reports whether lowering translates the opaque call c of the unsafe builtin
// name: unsafe.SliceData takes a gocpp slice (of C/C++ elements, []C.int, which is opaque too),
// and StringData a string; Add, Slice and String take C pointers and numbers as well as gocpp
// ones (the runtime's functions have both). It reports a user error if not.
func (g *gen) unsafeCallOK(c *ast.CallExpr, name string) bool {
	switch name {
	case "Sizeof", "Alignof", "Offsetof":
		return true // unsafeSize: the operand is not evaluated
	case "SliceData":
		if t := g.info.TypeOf(c.Args[0]); t == nil || !isSlice(t) {
			g.userError(c.Args[0].Pos(), "invalid argument: %s (C/C++ value) for unsafe.SliceData: it takes a slice, such as a []C.int", g.srcText(c.Args[0]))
			return false
		}
	case "StringData":
		g.userError(c.Args[0].Pos(), "invalid argument: %s (C/C++ value) for unsafe.StringData: it takes a string; for a C string use C.GoString(p)", g.srcText(c.Args[0]))
		return false
	}
	return true
}

// cPtrConvOK reports whether the operand e of the conversion p to a pointer type with a C/C++
// part ((*C.char)(e), cunsafe.go: cPointerType) is one that Go allows, as far as the declared
// type of e tells, and reports a user error, as cgo's, if it isn't: a C pointer converts to
// another pointer type only through unsafe.Pointer.
func (g *gen) cPtrConvOK(e ast.Expr, p *ast.CallExpr) bool {
	if kind, _ := g.cKind(e); kind == "pointer" {
		to := strings.TrimSuffix(strings.TrimPrefix(g.srcText(p.Fun), "("), ")") // (*C.char) as the type
		g.userError(e.Pos(), "cannot convert %s (C pointer) to type %s: convert it through unsafe.Pointer, %s(unsafe.Pointer(%s))", g.srcText(e), to, parenType(to), g.srcText(e))
		return false
	}
	return true
}

// parenType returns the type t of a conversion in parentheses where Go needs them: (*C.char).
func parenType(t string) string {
	if len(t) > 0 && t[0] == '*' {
		return "(" + t + ")"
	}
	return t
}

// cSliceOK reports whether the slice expression x of a C/C++ value (a C array, a std::array;
// gp::cgo::slice_of) is one that Go allows, as far as the declared type tells: a C pointer has
// no length to slice (cgo's error, and Go's for a pointer that is not to an array); a C++
// container has no slice expression either, which the C++ compiler reports (slice_of's
// static_assert).
func (g *gen) cSliceOK(x *ast.SliceExpr) bool {
	if kind, typ := g.cKind(x.X); kind == "pointer" {
		g.userError(x.Lbrack, "cannot slice %s (variable of type %s): a C pointer has no length; use unsafe.Slice(%s, n)", g.srcText(x.X), typ, g.srcText(x.X))
		return false
	}
	return true
}

// cSlice returns the C++ of the slice expression x of a C array or std::array:
// gp::cgo::slice_of(a) is the slice of all of its elements, over the C++ memory (it owns nothing:
// C++'s lifetime rules), and the bounds are gp::sub's, with Go's checks and panics.
func (fg *funcGen) cSlice(x *ast.SliceExpr) cppir.Expr {
	s := call("gp::cgo::slice_of", fg.expr(x.X))
	switch {
	case x.Low == nil && x.High == nil && x.Max == nil:
		return s
	case x.High == nil:
		return call("gp::sub", s, fg.intExpr(x.Low))
	case x.Max != nil:
		return call("gp::sub", s, fg.sliceLow(x), fg.intExpr(x.High), fg.intExpr(x.Max))
	}
	return call("gp::sub", s, fg.sliceLow(x), fg.intExpr(x.High))
}

// Pointers that own what they point to (Phase 04 README §2.7, step 4.10). A *T of a C/C++ type T that
// gocpp code declares (a variable, a parameter, a result, a field of a gocpp struct, an element of a
// slice, array or map: typeExpr) is a gp::cptr<T>: a gocpp pointer that owns or aliases what it
// points to, as Go's pointer does, converts to the raw T* of C and C++ (borrowed), and is assigned
// a raw pointer from C (which owns nothing: foreign_owner). &T{...} (a box built in place),
// &s.f of a gocpp struct and &xs[i] of a gocpp slice or array (the pointer shares the owner of the
// object) give one too. So the flows of Go code just work:
//
//	var head *lib.Node
//	for i := 0; i < 3; i++ { n := &lib.Node{v: i, next: head}; head = n } // each node lives on
//	p := &xs[1]; xs = append(xs, ...); *p = 3                                // the old array lives on
//
// What stays raw is C's: a member of a C struct (lib.Node.next), the pointer to a pointer of C
// memory (**C.char) and the arguments of C and C++ templates (std.vector[*lib.T]), and what C
// functions return (x := lib.f() is auto: a T*). A cptr that is stored into a member of a C struct is
// kept alive by the struct: the literal &lib.Node{next: head} (box_keep) and a later store
// n.next = head (store_keep, keepOf) record head's owner in the box of n, which a *T variable
// owns; where none does, the store is an error (noteStores). A read of a member gives a raw pointer,
// which converts to a cptr that owns the struct again (the registry of box_keep's objects, by
// address): x := cur.next is auto load(...) (ownedValue), so the variable owns it too.

// Kinds of owned pointers (ownedKind).
const (
	notOwned  = iota
	ownedTemp // &T{...}, or the address of a part of a temporary: nothing but the pointer keeps it alive
	ownedLoc  // the address of a part of a gocpp variable: &s.f, &xs[i]
	ownedVar  // a variable declared from an owned pointer
)

// ownedPath reports whether the operand x of &x is a part of a gocpp object that the address can
// share the owner of (a field of a gocpp struct, an element of a gocpp slice, array or map, behind
// a gocpp pointer), and the variable it is in, or nil when it is in a temporary (a call). A part of
// a C/C++ value (a member, an element of a C++ container or a C array) is not: its address is a raw
// C++ address, as &x of a C/C++ variable is.
func ownedPath(info *tinfo, x ast.Expr) (owned bool, root *types.Var) {
	for {
		switch y := ast.Unparen(x).(type) {
		case *ast.IndexExpr:
			if !gocppContainer(info.TypeOf(y.X)) {
				return false, nil
			}
			x = y.X
		case *ast.SelectorExpr:
			if s := info.Selections[y]; s == nil || s.Kind() != types.FieldVal {
				return false, nil
			}
			x = y.X
		case *ast.StarExpr:
			if t := info.TypeOf(y.X); t == nil || !complete(t) {
				return false, nil // a raw C pointer
			}
			x = y.X
		case *ast.Ident:
			v, _ := info.Uses[y].(*types.Var)
			return v != nil, v
		default:
			return true, nil
		}
	}
}

// ownedKind returns what kind of owned pointer the expression e is (notOwned for any other value).
func (g *gen) ownedKind(e ast.Expr) int {
	return g.ownedKindDepth(e, 0)
}

func (g *gen) ownedKindDepth(e ast.Expr, depth int) int {
	if depth > 8 || e == nil {
		return notOwned
	}
	switch x := ast.Unparen(e).(type) {
	case *ast.UnaryExpr:
		if x.Op != token.AND {
			return notOwned
		}
		switch y := ast.Unparen(x.X).(type) {
		case *ast.CompositeLit:
			if g.cxxLit(y) {
				return ownedTemp
			}
		case *ast.SelectorExpr, *ast.IndexExpr:
			if owned, root := ownedPath(g.info, y); owned {
				if root == nil {
					return ownedTemp
				}
				if isPkgLevel(root) {
					return notOwned // a package variable lives as long as the program
				}
				return ownedLoc
			}
		}
	case *ast.Ident:
		v, ok := g.info.Uses[x].(*types.Var)
		if !ok || v.IsField() || isPkgLevel(v) || g.typeExprOf(v) != nil {
			return notOwned // declared with a type: a raw pointer
		}
		if g.ownedKindDepth(g.initialValue(v), depth+1) != notOwned {
			return ownedVar
		}
	}
	return notOwned
}

// cOwnedAddr returns the C++ of &x for the part x of a gocpp object (ownedPath): a gp::cptr that
// shares the owner of the object, as the pointer of a typed part does (addr.go).
func (fg *funcGen) cOwnedAddr(e *ast.UnaryExpr) cppir.Expr {
	x := ast.Unparen(e.X)
	elem := fg.typeOfExpr(x)
	var p cppir.Expr
	switch y := x.(type) {
	case *ast.IndexExpr:
		if isSlice(fg.g.info.TypeOf(y.X)) {
			p = fg.elemAddr(e, y)
		} else {
			p = fg.addrOf(y)
		}
	default:
		p = fg.addrOf(x)
	}
	return construct(&cppir.TypeName{Name: "gp::cptr", Args: []cppir.Node{elem}}, p)
}

// cNew returns the C++ of &T{...} where T is a C/C++ type: gp::cgo::box_keep<T>([&](...) -> T { return
// T{...}; }, kept...): the object is built in place in a box (cBox, step 4.8: no copy or move, so a
// std.mutex works), and the box is the gocpp pointer (a gp::cptr) that owns it, as Go's &T{...}
// does. It converts to the raw T* for C, which only borrows it (README §2.7).
//
// A member of a C struct is a raw pointer: &lib.Node{v: 1, next: head} stores head's address
// there, and the new node must keep head's object alive, as Go's pointer would. The elements of
// the literal that are gocpp pointers (also those of nested struct literals) are evaluated once,
// as the arguments of box_keep, which holds their owners and gives them to the lambda (gp_k0...).
// Later stores into the node (n.next = head) are store_keep's (keepOf).
func (fg *funcGen) cNew(lit *ast.CompositeLit) cppir.Expr {
	typ := fg.g.typeExpr(lit.Type)
	var kept []ast.Expr
	fg.keptElems(lit, &kept)
	args := make([]cppir.Expr, len(kept))
	for i, e := range kept {
		args[i] = fg.expr(e)
	}
	type saved struct {
		e   ast.Expr
		old string
		had bool
	}
	var olds []saved
	var params []cppir.Param
	for i, e := range kept {
		name := "gp_k" + strconv.Itoa(i)
		e = ast.Unparen(e)
		old, had := fg.temps[e]
		olds = append(olds, saved{e, old, had})
		fg.temps[e] = name
		params = append(params, cppir.Param{Type: typeName("const auto&"), Name: name})
	}
	value := fg.compLit(lit)
	for _, o := range olds {
		if o.had {
			fg.temps[o.e] = o.old
		} else {
			delete(fg.temps, o.e)
		}
	}
	lam := cBoxLambda(typ, value)
	lam.Params = params
	return templateCall("gp::cgo::box_keep", []cppir.Node{typ}, append([]cppir.Expr{lam}, args...)...)
}

// keptElems adds to kept the elements of the literal lit (and of the struct literals in it) that are
// gocpp pointers: what a raw member of the new object would point to (cNew).
func (fg *funcGen) keptElems(lit *ast.CompositeLit, kept *[]ast.Expr) {
	for _, elt := range lit.Elts {
		if kv, ok := elt.(*ast.KeyValueExpr); ok {
			elt = kv.Value
		}
		switch x := ast.Unparen(elt).(type) {
		case *ast.CompositeLit:
			fg.keptElems(x, kept)
			continue
		}
		if fg.isGocppPointer(elt) && fg.g.info.Types[elt].Value == nil {
			*kept = append(*kept, elt)
		}
	}
}

// A keepInfo says that a store into a member of a C/C++ struct keeps the object alive: owner is the
// gocpp pointer that owns the struct (n of n.next = head) and path the types of the members that the
// store goes through (n.a.b.next: n.a and n.a.b), which must not be raw pointers.
type keepInfo struct {
	owner cppir.Expr
	path  []cppir.Node
}

// cStore describes a store into the left side lhs of an assignment, when it is a member or an element
// of a C/C++ object (n.next = p, n.slots[i] = p, *pp = p, lib.table[0] = p): base is the gocpp pointer
// that owns the object (n; nil when no pointer in the program does: a C/C++ local value, a raw pointer,
// a C global) and links the members between base and the target (n.a.b.next: n.a and n.a.b). member
// is false for any other left side: a variable, a field or an element of a gocpp value.
func (fg *funcGen) cStore(lhs ast.Expr) (member bool, base ast.Expr, links []ast.Expr) {
	member, base, links = fg.cStoreBase(lhs)
	if base != nil && hasCall(fg.g.info, base) {
		return member, nil, nil // evaluated twice, as the owner and in the member: let the user name it
	}
	return member, base, links
}

func (fg *funcGen) cStoreBase(lhs ast.Expr) (member bool, base ast.Expr, links []ast.Expr) {
	info := fg.g.info
	x := ast.Unparen(lhs)
	switch y := x.(type) {
	case *ast.SelectorExpr:
		if info.Selections[y] != nil || !fg.g.pkg.Opaque.IsOpaque(y) {
			return false, nil, nil
		}
		x = ast.Unparen(y.X)
		if fg.g.pkg.PromotedPath(y) != nil {
			// n.next of a member promoted from an embedded C struct (check/promoted.go): the same as
			// n.unode.next. Through a gocpp pointer n, the struct's block owns the field; in a value,
			// look further, as for the explicit path below. (A pointer inside the path, an embedded
			// *N, has no expression to own: promotedHint.)
			if t := info.TypeOf(y.X); t != nil {
				if _, ok := t.Underlying().(*types.Pointer); ok {
					return true, x, nil
				}
			}
		}
	case *ast.IndexExpr:
		if gocppContainer(info.TypeOf(y.X)) || !fg.g.pkg.Opaque.IsOpaque(y.X) {
			return false, nil, nil
		}
		x = ast.Unparen(y.X)
	case *ast.StarExpr:
		if !fg.g.pkg.Opaque.IsOpaque(y.X) {
			return false, nil, nil
		}
		if fg.g.mayOwn(y.X) {
			return true, y.X, nil // *n of a *lib.Node variable: the object n owns
		}
		return true, nil, nil // *pp: the object is C's
	default:
		return false, nil, nil
	}
	for range 16 {
		if fg.g.mayOwn(x) {
			return true, x, links
		}
		switch y := x.(type) {
		case *ast.SelectorExpr:
			if info.Selections[y] != nil {
				// A field of a gocpp struct (s.c.next with c a C struct): through a gocpp
				// pointer the struct's block owns the field; in a struct value, look further.
				if t := info.TypeOf(y.X); t != nil {
					if _, ok := t.Underlying().(*types.Pointer); ok {
						return true, ast.Unparen(y.X), links
					}
				}
				x = ast.Unparen(y.X)
				continue
			}
			if fg.g.pkg.PromotedPath(y) != nil {
				// hh.in of hh.in.p, a member promoted from an embedded C struct: through a gocpp
				// pointer hh the struct's block owns it.
				if t := info.TypeOf(y.X); t != nil {
					if _, ok := t.Underlying().(*types.Pointer); ok {
						return true, ast.Unparen(y.X), links
					}
				}
			}
			links = append(links, x)
			x = ast.Unparen(y.X)
		case *ast.IndexExpr:
			if t := info.TypeOf(y.X); t != nil {
				if _, ok := t.Underlying().(*types.Slice); ok {
					return true, ast.Unparen(y.X), links // nodes[i].next: the slice's block owns the element
				}
			}
			if gocppContainer(info.TypeOf(y.X)) {
				return true, nil, nil
			}
			links = append(links, x)
			x = ast.Unparen(y.X)
		case *ast.StarExpr:
			if fg.g.mayOwn(y.X) {
				return true, y.X, links
			}
			return true, nil, nil
		default:
			return true, nil, nil
		}
	}
	return true, nil, nil
}

// isMemberRead reports whether e reads a member or an element of a C/C++ object (cur.next,
// n.slots[i], *pp): an opaque value that may be a pointer to a C struct that gocpp owns.
func (g *gen) isMemberRead(e ast.Expr) bool {
	e = ast.Unparen(e)
	if _, ok := g.cRefOf(e); ok {
		return false // a C/C++ name (lib.MAC, lib.table[0] is a name's element: below), not a member
	}
	switch x := e.(type) {
	case *ast.SelectorExpr:
		return g.info.Selections[x] == nil && g.pkg.Opaque.IsOpaque(x)
	case *ast.IndexExpr:
		return !gocppContainer(g.info.TypeOf(x.X)) && g.pkg.Opaque.IsOpaque(x)
	case *ast.StarExpr:
		return g.pkg.Opaque.IsOpaque(x.X)
	}
	return false
}

// loadedVar reports whether e is a variable declared from a member read (next := cur.next): the
// declaration is auto load(...), a gp::cptr when the member is a pointer to a C struct.
func (g *gen) loadedVar(e ast.Expr) bool {
	id, ok := ast.Unparen(e).(*ast.Ident)
	if !ok {
		return false
	}
	v, ok := g.info.Uses[id].(*types.Var)
	if !ok || v.IsField() || isPkgLevel(v) || g.typeExprOf(v) != nil {
		return false
	}
	return g.isMemberRead(g.initialValue(v))
}

// mayOwn reports whether e may be a pointer that owns the struct it points to: a gp::cptr, or a
// variable declared from a member read (loadedVar: store_keep tells when it compiles).
func (g *gen) mayOwn(e ast.Expr) bool {
	return g.isOwnerPtr(e) || g.loadedVar(e)
}

// isOwnerPtr reports whether the value e is a gocpp pointer to a C/C++ struct (a gp::cptr), not the
// raw pointer of a **C.T or of a C call's result: the pointer that owns the object it points to.
func (g *gen) isOwnerPtr(e ast.Expr) bool {
	if !g.isCPtrValue(e) {
		return false
	}
	if star, ok := ast.Unparen(g.valueTypeExpr(e)).(*ast.StarExpr); ok {
		if _, nested := ast.Unparen(star.X).(*ast.StarExpr); nested {
			return false
		}
	}
	return true
}

// noteStores finds the stores of the assignment s into members and elements of C/C++ objects (cStore)
// whose value is a gocpp pointer. When a pointer owns the object, lhs() makes the store a store_keep
// (keepOf). When none does, nothing could keep the pointer's object alive after the store: it is an
// error, as Go's pointer would have kept it.
// promotedHint returns the hint for an error about a store into m.next, a member promoted from an
// embedded C/C++ type: "; or write the path of the embedded fields, m.N.unode.next, which names the
// pointer that owns the struct", or "" for any other target.
func (g *gen) promotedHint(lhs ast.Expr) string {
	sel, ok := ast.Unparen(lhs).(*ast.SelectorExpr)
	if !ok {
		return ""
	}
	path := g.pkg.PromotedPath(sel)
	if path == nil {
		return ""
	}
	t := g.info.TypeOf(sel.X)
	text := g.srcText(sel.X)
	for _, i := range path {
		if p, ok := under(t).(*types.Pointer); ok {
			t = p.Elem()
		}
		f := under(t).(*types.Struct).Field(i)
		text += "." + f.Name()
		t = f.Type()
	}
	return "; or write the path of the embedded fields, " + text + "." + sel.Sel.Name + ", which names the pointer that owns the struct"
}

func (fg *funcGen) noteStores(s *ast.AssignStmt) {
	if s.Tok != token.ASSIGN || len(s.Lhs) != len(s.Rhs) || fg.storesNoted[s] {
		return
	}
	fg.storesNoted[s] = true
	for i, lhs := range s.Lhs {
		if isBlank(lhs) {
			continue
		}
		member, base, _ := fg.cStore(lhs)
		switch {
		case !member:
		case base != nil:
			fg.memberStores[ast.Unparen(lhs)] = s.Rhs[i]
		case fg.g.isCPtrValue(s.Rhs[i]):
			fg.g.userError(s.Rhs[i].Pos(), "cannot keep the object alive: %s is stored into %s, which no *T variable of the program owns (a C/C++ local value, a pointer from C, or a pointer inside a C struct): declare the struct n := &T{...} and store through n, or write the literal &T{member: ...}%s",
				fg.g.srcText(s.Rhs[i]), fg.g.srcText(lhs), fg.g.promotedHint(lhs))
		}
	}
}

// keepOf returns the keepInfo of the store lhs = rhs, or nil when it needs none: the value is not a
// gocpp pointer or nil, or no pointer owns the struct (noteStores).
func (fg *funcGen) keepOf(lhs, rhs ast.Expr) *keepInfo {
	member, base, links := fg.cStore(lhs)
	if !member || base == nil {
		return nil
	}
	if tv, ok := fg.g.info.Types[ast.Unparen(rhs)]; !(ok && tv.IsNil()) && !fg.isGocppPointer(rhs) && !fg.g.isMemberRead(rhs) && !fg.g.loadedVar(rhs) {
		return nil // (a member read or a variable from one may be a pointer: store_keep tells)
	}
	k := &keepInfo{owner: fg.expr(base)}
	for _, l := range links {
		k.path = append(k.path, typeName("decltype("+cppir.ExprString(fg.expr(l))+")"))
	}
	return k
}

// isCPtrValue reports whether the value e is a gocpp pointer to a C/C++ type (a gp::cptr, or the raw
// pointer that one of a **C.T is) as far as its declaration tells: a variable, parameter, field or
// result declared with a *C.T type, or a variable that takes its type from &T{...}, &s.f or
// &xs[i] (not one that takes it from a C call: that is a raw pointer).
func (g *gen) isCPtrValue(e ast.Expr) bool {
	if g.ownedKind(e) != notOwned {
		return true
	}
	if u, ok := ast.Unparen(e).(*ast.UnaryExpr); ok && u.Op == token.AND {
		return false // &x of a C/C++ variable: a raw address
	}
	if id, ok := ast.Unparen(e).(*ast.Ident); ok {
		if v, ok := g.info.Uses[id].(*types.Var); ok && g.typeExprOf(v) == nil {
			// p := &x of a C/C++ variable: a raw address
			if u, ok := ast.Unparen(g.initialValue(v)).(*ast.UnaryExpr); ok && u.Op == token.AND {
				return false
			}
		}
	}
	star, ok := ast.Unparen(g.valueTypeExpr(e)).(*ast.StarExpr)
	return ok && g.isCType(star.X)
}

// rawPointerArgs turns the arguments of a call of a C/C++ function that are gocpp pointers
// (also unsafe.Pointer and the results of unsafe.SliceData, StringData and Add) into their raw
// addresses, p.gp_get(): args are the C++ of list. A gocpp pointer converts to T* implicitly
// (borrowed: the function may only use it during the call), but not through a C function's `...`
// (printf("%p", p), ioctl, fcntl, curl_easy_setopt: a class object there is an error in clang
// and, with -Werror=conditionally-supported, in g++), and an overload pair f(bool) + f(void*) is
// ambiguous for it. gp_get is a nil-safe raw address, null for nil. A pointer that goes to a C++
// template (std::make_pair(p, 1), a member push_back(p)) stays what it is: only the calls of C/C++
// functions that are not templates come here (cCall).
func (fg *funcGen) rawPointerArgs(list []ast.Expr, args []cppir.Expr) {
	for i, a := range list {
		switch {
		case i >= len(args) || fg.g.isPtrToUnsafePointer(a) || fg.isOutPtrArg(a):
		case fg.g.isCPtrValue(a):
			args[i] = call("gp::cgo::raw", args[i]) // a gp::cptr, or a raw pointer: whatever it is
		case fg.isGocppPointer(a):
			args[i] = &cppir.Call{Fun: &cppir.Member{X: args[i], Name: "gp_get"}}
		}
	}
}

// isGocppPointer reports whether e is a gocpp pointer or unsafe.Pointer value: of such a type,
// or the call of an unsafe function that gives one (its Go type may be a C pointer's).
func (fg *funcGen) isGocppPointer(e ast.Expr) bool {
	e = ast.Unparen(e)
	if fg.g.isCPtrValue(e) {
		return true // a gp::cptr (or the raw pointer of a **C.T)
	}
	if tv, ok := fg.g.info.Types[e]; ok && tv.Value == nil && !tv.IsNil() && complete(tv.Type) && !fg.g.pkg.Opaque.IsOpaque(e) {
		return isPointer(tv.Type) || isUnsafePointer(tv.Type)
	}
	if c, ok := e.(*ast.CallExpr); ok {
		switch fg.g.unsafeBuiltin(c) {
		case "SliceData", "StringData", "Add":
			return true
		}
	}
	return false
}

// boxedAutoType returns the C++ type of the boxed variable v, declared without a type from a
// value with C/C++ parts (pts := [3]lib.Point{...}, whose address, or the address of an element,
// is taken): from the type of its initial value. The box needs the type: gp::ptr<auto> is not C++.
func (fg *funcGen) boxedAutoType(v *types.Var, id *ast.Ident) cppir.Type {
	if te := fg.g.valueTypeExpr(fg.g.initialValue(v)); te != nil {
		return fg.g.typeExpr(te)
	}
	fg.g.unsupported(id.Pos(), "taking the address of "+v.Name()+", a variable of a C/C++ value without a declared type: declare it, var "+v.Name()+" T = ...")
	return typeName("gp_unsupported")
}

// cptrType returns gp::cptr<elem>, the C++ type of a gocpp pointer to the C/C++ type elem.
func cptrType(elem cppir.Type) cppir.Type {
	return &cppir.TypeName{Name: "gp::cptr", Args: []cppir.Node{elem}}
}

// outPtrTarget returns the variable p of the argument &p of a C/C++ function, when p is declared
// with a pointer to a C/C++ type (var db *sqlite3.sqlite3; sqlite3_open(name, &db)), or nil: the
// parameter is a T** that the function writes. p is a gp::cptr<T> (16 bytes), so &p is no T**:
// the argument is gp::cgo::out_ptr(p), a T* that p gets when the statement ends.
func (fg *funcGen) outPtrTarget(a ast.Expr) ast.Expr {
	u, ok := ast.Unparen(a).(*ast.UnaryExpr)
	if !ok || u.Op != token.AND {
		return nil
	}
	switch x := ast.Unparen(u.X).(type) {
	case *ast.Ident, *ast.SelectorExpr:
		star, ok := ast.Unparen(fg.g.valueTypeExpr(x)).(*ast.StarExpr)
		if !ok || !fg.g.isCType(star.X) {
			return nil
		}
		if _, nested := ast.Unparen(star.X).(*ast.StarExpr); nested {
			return nil // a **C.T variable: its address is a raw T***
		}
		return x
	}
	return nil
}

// isOutPtrArg reports whether a is the argument &p of outPtrTarget.
func (fg *funcGen) isOutPtrArg(a ast.Expr) bool { return fg.outPtrTarget(a) != nil }
