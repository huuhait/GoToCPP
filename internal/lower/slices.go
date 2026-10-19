package lower

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"strconv"

	"github.com/huuhait/gocpp/internal/cppir"
)

// Slices (Phase 03 README §2.3, step 3.5; the runtime is gocpp/slice.hpp):
//
//	Go                          C++
//	[]T                         gp::slice<T>
//	var s []T, s = nil          gp::slice<T> s{};  s = nullptr
//	s == nil, s != nil          s == nullptr, s != nullptr
//	[]T{a, b}, []T{}            gp::slice_of<T>({a, b}), gp::slice_of<T>({})
//	[]T{5: a, b}                gp::slice_keyed<T>(7, {{5, a}, {6, b}}) (keyed: the rest zero)
//	make([]T, n), (n, c)        gp::make_slice<T>(n), gp::make_slice<T>(n, c)
//	len(s), cap(s)              gp::len(s), gp::cap(s)
//	s[i]                        s[i] (a T&: read, assigned, ++, op=), bounds checked
//	s[a:b], s[a:b:c]            gp::sub(s, a, b), gp::sub(s, a, b, c) (ops.go: slice)
//	append(s, a, b)             gp::append(s, a, b)
//	append(s, t...)             gp::append_slice(s, t), also append([]byte, string...)
//	copy(dst, src)              gp::copy(dst, src), also copy([]byte, string)
//	clear(s)                    gp::clear(s)
//	&s[i]                       gp::ptr<T>(s.gp_owner(), &s[i]) (addr.go)
//	for i, v := range s         for (auto gp_it0 = gp::range_slice(s); gp::range_next(gp_it0);)
//	                            (control.go: rangeIter)
//	[]byte(s), []rune(s)        gp::to_bytes(s), gp::to_runes(s) (to_bytes_const, to_runes_const for a constant s)
//	string(b), string(runes)    gp::to_string(b), gp::to_string(runes)
//
// A slice is a class type (gp::slice holds an owner), so a call of a user function with a slice
// argument is qualified (funcCallName), and a temporary is moved into its variable.
//
// s = append(s, x) and s = s[a:b] move s into the runtime's call: the call takes its slice by
// value, so a copy would cost an atomic increment and decrement of the array's count, for each
// append in a loop (1e7 appends: 130 ms moved, 170 to 230 ms copied). The move is a cast,
// static_cast<gp::slice<T>&&>(s) (moved, decl.go), and only where nothing else in the call
// can see s while it is moved out (nil): see markMoves.
//
// Go's order of evaluation (order.go): an element of a slice is memory that any call can change
// (through another slice of the same array), so s[i] is read after the calls of its scope, as gc
// does (and is not copied as an argument of fmt's ...any: it is addressable); append, copy and
// make are evaluated in their places among the calls (gc copies them into temporaries), and so
// are []byte(s) and []rune(s), while string(b) reads b's bytes where it is, after the calls; the
// elements of a composite literal are ordered like the operands of an operator.

// compositeLit returns the C++ of a composite literal. Step 3.5 has slice literals: the elements
// are a braced list, which C++ evaluates in order, each converted to T by the list (a constant
// is written in T's type by go/types' record of it). A keyed literal, []int{5: 1, 2}, gives each
// element its index (a key, or the index after the element before it), and its length is one
// more than the largest index.
func (fg *funcGen) compositeLit(e *ast.CompositeLit) cppir.Expr {
	t := fg.g.litType(e) // []T of an elided &[]T{...} too
	if isMap(t) {
		return fg.mapLit(e) // step 3.7, maps.go
	}
	_, ok := under(t).(*types.Slice) // also of a named slice type (litValue converts it)
	if !ok {
		if e.Type != nil && fg.g.isGenericTypeExpr(e.Type) {
			fg.g.notYet(e.Pos(), "an instance of a generic type with a C/C++ type argument ("+types.ExprString(e.Type)+") is not supported yet: the type arguments of a generic type are Go types")
			return placeholder
		}
		fg.g.unsupported(e.Lbrace, "composite literals of type "+types.TypeString(t, types.RelativeTo(fg.g.pkg.Types)))
		return placeholder
	}
	elem := fg.g.litPartType(e, t, elemOf) // (also C/C++ elements: []C.int, clits.go)
	keyed := false
	for _, elt := range e.Elts {
		if _, ok := elt.(*ast.KeyValueExpr); ok {
			keyed = true
		}
	}
	if !keyed {
		return templateCall("gp::slice_of", []cppir.Node{elem}, &cppir.InitList{Elems: fg.exprs(e.Elts, false)})
	}
	var pairs []cppir.Expr
	var index, n int64
	for _, elt := range e.Elts {
		if kv, ok := elt.(*ast.KeyValueExpr); ok {
			index, _ = constant.Int64Val(constant.ToInt(fg.constValue(kv.Key)))
			elt = kv.Value
		}
		pairs = append(pairs, &cppir.InitList{Elems: []cppir.Expr{lit(strconv.FormatInt(index, 10)), fg.expr(elt)}})
		index++
		n = max(n, index)
	}
	return templateCall("gp::slice_keyed", []cppir.Node{elem}, lit(strconv.FormatInt(n, 10)), &cppir.InitList{Elems: pairs})
}

// sliceBuiltin returns the C++ of a call of make, append, copy or clear on slices.
func (fg *funcGen) sliceBuiltin(c *ast.CallExpr, name string) cppir.Expr {
	arg := c.Args[0]
	t := fg.g.info.TypeOf(arg)
	if core := fg.g.cxxCore(t); core != nil && name != "make" {
		t = core // S ~[]E that C++ deduces (generics.go)
	}
	if !isSlice(t) {
		what := map[string]string{"make": "make of ", "append": "append to ", "copy": "copy to ", "clear": "clear of "}[name]
		phase := ""
		if _, ok := types.Unalias(t).(*types.TypeParam); ok {
			phase = " (Phase 07)"
		} else if _, ok := under(t).(*types.Chan); ok {
			phase = " (Phase 08)"
		}
		fg.g.unsupported(c.Pos(), what+types.TypeString(t, types.RelativeTo(fg.g.pkg.Types))+phase)
		return placeholder
	}
	fg.g.noteSliceBuiltin(c, name) // why.go
	switch name {
	case "make":
		// make([]T, n) and make([]T, n, c): the type is the first argument.
		var elem cppir.Type
		if et := under(t).(*types.Slice).Elem(); complete(et) {
			elem = fg.g.cxxType(et, arg.Pos())
		} else if at, ok := ast.Unparen(arg).(*ast.ArrayType); ok {
			elem = fg.g.typeExpr(at.Elt) // make([]C.int, n): from the type expression
		} else {
			elem = fg.g.cxxType(et, arg.Pos()) // (reports it)
		}
		args := make([]cppir.Expr, len(c.Args)-1)
		for i, a := range c.Args[1:] {
			if t := fg.g.info.TypeOf(a); fg.g.info.Types[a].Value == nil && (t == nil || !complete(t)) {
				// A size that is a C number, enum or bool (C.size_t, an enum): as cgo, any integer.
				args[i] = templateCall("gp::cgo::as", []cppir.Node{typeName("int64_t")}, fg.expr(a))
				continue
			}
			args[i] = fg.intExpr(a)
		}
		return templateCall("gp::make_slice", []cppir.Node{elem}, args...)
	case "append":
		s := fg.movedExpr(arg)
		if c.Ellipsis.IsValid() {
			// append(s, t...): t is a slice, or a string for a []byte s. go/types leaves a nil t
			// untyped: it is a nil slice of s's type, gp::slice<T>().
			if fg.g.info.Types[c.Args[1]].IsNil() {
				return call("gp::append_slice", s, construct(fg.g.cxxType(t, arg.Pos())))
			}
			return call("gp::append_slice", s, fg.exactExpr(c.Args[1]))
		}
		// The template converts each element to T, so a constant needs no exact type.
		return call("gp::append", append([]cppir.Expr{s}, fg.exprs(c.Args[1:], false)...)...)
	case "copy":
		return call("gp::copy", fg.expr(arg), fg.expr(c.Args[1]))
	default: // clear
		return call("gp::clear", fg.expr(arg))
	}
}

// sliceConversion returns the C++ of a conversion to or from a slice type: []T(nil) is
// gp::slice<T>(), []T(s) of a slice of the same C++ type is s (identical types: a named slice
// type converts as its underlying type, namedConversion), and []byte(s), []rune(s), string(b)
// and string(runes) are the runtime's copying conversions (slice.hpp).
func (fg *funcGen) sliceConversion(c *ast.CallExpr, to, from types.Type, x cppir.Expr) cppir.Expr {
	arg := c.Args[0]
	qual := types.RelativeTo(fg.g.pkg.Types)
	if isSlice(to) {
		typ := fg.g.cxxType(to, c.Pos())
		switch {
		case fg.g.info.Types[arg].IsNil():
			return construct(typ)
		case isSlice(from) && cppir.TypeString(typ) == cppir.TypeString(fg.g.cxxType(from, arg.Pos())):
			return x
		case isString(from):
			// gc gives a constant string an exact array (cap == len); any other string goes
			// through rawbyteslice and rawruneslice, which round the capacity up to a size
			// class (to_bytes, to_runes).
			suffix := ""
			if fg.g.info.Types[arg].Value != nil {
				suffix = "_const"
			}
			switch elemKind(to) {
			case types.Uint8:
				return call("gp::to_bytes"+suffix, x)
			case types.Int32:
				return call("gp::to_runes"+suffix, x)
			}
		}
	} else if isString(to) {
		switch elemKind(from) {
		case types.Uint8, types.Int32:
			return call("gp::to_string", x)
		}
	}
	fg.g.unsupported(c.Pos(), "the conversion from "+types.TypeString(from, qual)+" to "+types.TypeString(to, qual))
	return placeholder
}

// elemKind returns the kind of the element type of the slice type t if it is a basic type
// (byte is Uint8, rune Int32), or types.Invalid.
func elemKind(t types.Type) types.BasicKind {
	st, ok := under(t).(*types.Slice)
	if !ok {
		return types.Invalid
	}
	if b, ok := types.Unalias(st.Elem()).(*types.Basic); ok {
		return b.Kind()
	}
	return types.Invalid
}

// movedExpr returns the C++ of the slice e, moved (static_cast<gp::slice<T>&&>(e)) if markMoves
// chose it.
func (fg *funcGen) movedExpr(e ast.Expr) cppir.Expr {
	x := fg.expr(e)
	if fg.moves[e] {
		if t := fg.g.info.TypeOf(e); !complete(t) {
			return x // a slice of C/C++ values, []C.int: no C++ type to name; a copy shares the array
		}
		return moved(fg.g.cxxType(fg.g.info.TypeOf(e), e.Pos()), x)
	}
	return x
}

// markMoves chooses whether the assignment lhs = rhs moves the slice of its value: in
// s = append(s, ...) and s = s[a:b] (also s[a:], s[:b] and s[a:b:c]), where s is the same
// variable on both sides (a local or package variable, *p, or an element v[i] of a variable v
// with an index that is a constant or a variable), the call can take s's array without
// touching its count, since s gets the result anyway. The move happens when the call's
// parameter is initialized, which C++ may do before or after its other arguments are evaluated,
// so it is only made when no other argument can see s:
//   - no other argument names the variable that s is or is in (s = append(s, s[0]),
//     s = s[:len(s)-1] and g[k] = append(g[k], g[j][0]) copy; an index, k, may be named);
//   - and, unless s is a local variable that is not boxed and not a named result, no other
//     argument has an effect but reads of other variables (order.go's items): a call, *p or an
//     element of a slice could reach s through a pointer or a shared array (p := &ss[0];
//     *p = append(*p, ss[0][0])), and a panic would leave a nil where a deferred function
//     (step 3.13) or a recover (Phase 05) can see it. For the same reason such an s is not
//     moved into gp::sub, whose bounds check panics after the move (gs = gs[i:]); append
//     panics only when the new length can't be allocated. A local variable is gone after a
//     panic, and only a call that it is passed to can read it.
//
// gc does the same with its own rule (walk/order.go: an append to the same safe expression is
// done in place). It runs after the statement's order (order.go), so the arguments that went
// into temporaries are only their names.
func (fg *funcGen) markMoves(lhs, rhs ast.Expr) {
	var s ast.Expr
	var others []ast.Expr
	switch r := ast.Unparen(rhs).(type) {
	case *ast.CallExpr:
		if !fg.isBuiltin(r, "append") || !fg.ordered[r] || fg.temps[r] != "" {
			return
		}
		s, others = r.Args[0], r.Args[1:]
	case *ast.SliceExpr:
		if !isSlice(fg.g.info.TypeOf(r.X)) || fg.temps[r] != "" || r.Low == nil && r.High == nil && r.Max == nil {
			return
		}
		s = r.X
		for _, b := range []ast.Expr{r.Low, r.High, r.Max} {
			if b != nil {
				others = append(others, b)
			}
		}
	default:
		return
	}
	if !isSlice(fg.g.info.TypeOf(s)) || fg.temps[ast.Unparen(s)] != "" || !fg.sameVar(lhs, s) {
		return
	}
	names := fg.usedNames([]ast.Expr{baseVar(s)})
	for name := range fg.usedNames(others) {
		if names[name] {
			return
		}
	}
	if !fg.plainLocal(lhs) {
		if _, sub := ast.Unparen(rhs).(*ast.SliceExpr); sub {
			return
		}
		for _, o := range others {
			for _, it := range fg.analyze(false, []ast.Expr{o}).items {
				if _, ok := it.e.(*ast.Ident); !ok || it.eff.call || it.eff.panics {
					return
				}
			}
		}
	}
	fg.moves[s] = true
}

// appendInPlace returns the append call of the assignment s if it is s = append(s, ...), where
// s is the same variable on both sides (sameVar), or nil. gc evaluates such an append in place,
// not in a temporary of its own (walk/order.go: an OAPPEND whose left side is the same safe
// expression as its first argument), so it is not a unit of the order: the call can then move
// s (markMoves), also for a package slice, g[k] = append(g[k], x), where a temporary would copy
// the result back.
func (fg *funcGen) appendInPlace(s ast.Stmt) *ast.CallExpr {
	a, ok := s.(*ast.AssignStmt)
	if !ok || a.Tok != token.ASSIGN || len(a.Lhs) != 1 || len(a.Rhs) != 1 || isBlank(a.Lhs[0]) {
		return nil
	}
	c, ok := ast.Unparen(a.Rhs[0]).(*ast.CallExpr)
	if !ok || !fg.isBuiltin(c, "append") || !fg.sameVar(a.Lhs[0], c.Args[0]) {
		return nil
	}
	return c
}

// storedExpr returns the C++ of rhs, the value of a plain assignment: moved
// (static_cast<T&&>(gp_t0)) if it is a temporary of the order (order.go) of a class type,
// which is used only here, and copied first if the store could free it (sharedCopy).
func (fg *funcGen) storedExpr(rhs ast.Expr) cppir.Expr {
	x := fg.expr(rhs)
	if _, ok := fg.temps[ast.Unparen(rhs)]; ok {
		t := fg.g.info.TypeOf(rhs)
		if to, ok := fg.g.implicit[rhs]; ok {
			t = to // the temporary holds the value converted to the target's type (step 3.9)
		}
		if t != nil && complete(t) && isClass(t) && !fg.g.info.Types[rhs].IsNil() { // (not a C/C++ value: auto)
			return moved(fg.g.cxxType(t, rhs.Pos()), x)
		}
	}
	return fg.sharedCopy(rhs, fg.cMoved(rhs, x)) // n = n.kids[0] (structs.go); a C/C++ value's temporary
}

// baseVar returns the variable that the slice s (a variable, *p, v[i] or v.f, sameVar) is or is
// in: s, p, v's.
func baseVar(s ast.Expr) ast.Expr {
	for {
		switch x := ast.Unparen(s).(type) {
		case *ast.StarExpr:
			s = x.X
		case *ast.IndexExpr:
			s = x.X
		case *ast.SelectorExpr:
			s = x.X
		default:
			return s
		}
	}
}

// sameVar reports whether a and b are the same variable that can be read twice with the same
// result: the same variable x, *x of the same x, x[i] of the same x (a variable, *x or x[i]
// again) and the same constant or variable i, or the same field x.f of the same x.
func (fg *funcGen) sameVar(a, b ast.Expr) bool {
	switch x := ast.Unparen(a).(type) {
	case *ast.Ident:
		y, ok := ast.Unparen(b).(*ast.Ident)
		if !ok {
			return false
		}
		v, ok := fg.g.info.Uses[x].(*types.Var)
		return ok && v == fg.g.info.Uses[y]
	case *ast.StarExpr:
		y, ok := ast.Unparen(b).(*ast.StarExpr)
		return ok && fg.sameVar(x.X, y.X)
	case *ast.IndexExpr:
		y, ok := ast.Unparen(b).(*ast.IndexExpr)
		if !ok || !isSlice(fg.g.info.TypeOf(x.X)) || !fg.sameVar(x.X, y.X) {
			return false
		}
		cx, cy := fg.constValue(x.Index), fg.constValue(y.Index)
		if cx != nil || cy != nil {
			return cx != nil && cy != nil && constant.Compare(cx, token.EQL, cy)
		}
		return fg.sameVar(x.Index, y.Index)
	case *ast.SelectorExpr:
		// The same field of the same variable: x.f, p.f, s[i].f (structs.go).
		y, ok := ast.Unparen(b).(*ast.SelectorExpr)
		if !ok || fg.fieldSel(x) == nil || fg.fieldSel(y) == nil {
			return false
		}
		return fg.fieldSel(x).Obj() == fg.fieldSel(y).Obj() && fg.sameVar(x.X, y.X)
	}
	return false
}

// plainLocal reports whether e is a local variable that is not boxed and is not a named result
// (markMoves).
func (fg *funcGen) plainLocal(e ast.Expr) bool {
	id, ok := ast.Unparen(e).(*ast.Ident)
	if !ok {
		return false
	}
	v, ok := fg.g.info.Uses[id].(*types.Var)
	if !ok || isPkgVar(v) || fg.isBoxed(v) || fg.fn == nil {
		return false
	}
	res := fg.fn.sig.Results()
	for i := range res.Len() {
		if res.At(i) == v {
			return false
		}
	}
	return true
}
