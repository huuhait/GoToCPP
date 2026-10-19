package lower

import (
	"go/ast"
	"go/types"
	"sort"

	"github.com/huuhait/gocpp/internal/cppir"
	"github.com/huuhait/gocpp/internal/typedinterop/ctype"
	"github.com/huuhait/gocpp/internal/typedinterop/ftype"
	"github.com/huuhait/gocpp/internal/typedinterop/probe"
)

// Keyed literals of C/C++ aggregates as designated initializers (Phase 12 step 12.6).
//
//	lib.Rect{h: 480, x: 1, w: 640}        ::Rect{.x = 1, .w = 640, .h = 480}
//
// C++20 designated initializers must follow the declaration order of the fields (g++ and clang
// both reject another order), while a Go literal lists the fields in any order. Opaque mode knows
// no declaration order, so it stores the fields one by one in a lambda (cvalues of clits.go).
// With the record's fields from the probe (probe.Record, in declaration order, with the members
// of anonymous unions and structs flattened) the typed literal is one braced list:
//
//   - the keys are sorted by the position of the field; Go evaluates the values in source order,
//     so when the order changes and two or more values call something, the calls go into locals of a
//     lambda in source order first (typedLitHoist); a value without a call (a constant, a
//     variable, a field) can be evaluated anywhere, as in Go, whose order is unspecified between
//     a variable read and a call;
//   - a nested literal of a C/C++ type is a designated initializer of its own (the same
//     function, from cLit), and a Go array literal for a C array field is the braced list of
//     its elements;
//   - bit-fields, members of anonymous unions and structs, unions (one key) and aggregates with base
//     classes (their bases are value-initialized, as in the assignment form) are written the same way;
//
// The literal stays in the assignment form (nil from typedFieldsLit) whenever the type is not an
// aggregate, not complete, a key is not a public field of the record itself, two members of
// one union are named, or the probe has no answer for the literal.

// typedFieldsLit returns the designated initializer for the keyed literal e of the C/C++ type
// typ (cLit's litFields case), or nil when the typed information is not certain.
func (fg *funcGen) typedFieldsLit(e *ast.CompositeLit, typ cppir.Type) cppir.Expr {
	g := fg.g
	v := g.tvalOf(e)
	if v == nil || v.Kind != ftype.Record || typ == nil {
		return nil
	}
	rep := g.interop.typed.rep
	rec := rep.Decls.Records[rep.Mapper.Canonical(v.Type)]
	if rec == nil || !rec.Complete || !rec.Traits["aggregate"] {
		return nil
	}
	type item struct {
		field int
		kv    *ast.KeyValueExpr
		src   int
	}
	items := make([]item, 0, len(e.Elts))
	unions := 0
	for i, elt := range e.Elts {
		kv := elt.(*ast.KeyValueExpr)
		name := kv.Key.(*ast.Ident).Name
		idx := -1
		for j := range rec.Fields {
			if rec.Fields[j].Name == name {
				idx = j
				break
			}
		}
		if idx < 0 || rec.Fields[idx].Access != probe.Public {
			return nil // not a member of the record itself (a base's, a typo): the assignment form
		}
		if rec.Fields[idx].Union {
			unions++
		}
		items = append(items, item{idx, kv, i})
	}
	if unions > 1 || rec.Tag == "union" && len(items) != 1 {
		return nil // two members of one union cannot both be initialized
	}
	inOrder := sort.SliceIsSorted(items, func(a, b int) bool { return items[a].field < items[b].field })
	sort.SliceStable(items, func(a, b int) bool { return items[a].field < items[b].field })

	// The value of each key, in source order (the order of e.Elts).
	vals := make([]cppir.Expr, len(e.Elts))
	for _, it := range items {
		x, ok := fg.typedFieldValue(rec, &rec.Fields[it.field], it.kv.Value)
		if !ok {
			return nil
		}
		vals[it.src] = x
	}

	var pre []cppir.Stmt
	if !inOrder {
		// The calls of the values run in Go's order (source order), not in the declaration order of
		// the braced list: with two or more calls, each goes into a local first.
		calls := 0
		for _, elt := range e.Elts {
			if g.hasCall(elt.(*ast.KeyValueExpr).Value) {
				calls++
			}
		}
		if calls >= 2 {
			hoist := func(x cppir.Expr) cppir.Expr {
				t := fg.names.temp("t")
				pre = append(pre, varDecl(cppir.Pos{}, typeName("auto"), t, x))
				return movedName(t)
			}
			for i, elt := range e.Elts {
				val := elt.(*ast.KeyValueExpr).Value
				if !g.hasCall(val) {
					continue
				}
				// An array field's value is an unnamed braced list, which `auto` cannot hold: each of its
				// elements that calls something is hoisted, in order.
				if il, ok := vals[i].(*cppir.InitList); ok && il.Type == nil {
					lit := ast.Unparen(val).(*ast.CompositeLit)
					for k, el := range lit.Elts {
						if g.hasCall(el) {
							il.Elems[k] = hoist(il.Elems[k])
						}
					}
					continue
				}
				vals[i] = hoist(vals[i])
			}
		}
	}
	elems := make([]cppir.Expr, len(items))
	for i, it := range items {
		name := rec.Fields[it.field].Name
		g.noteCName(name, it.kv.Key.Pos())
		elems[i] = g.cNode(&cppir.Designated{Name: name, X: vals[it.src]}).(*cppir.Designated)
	}
	list := &cppir.InitList{Type: typ, Elems: elems}
	if len(pre) == 0 {
		return list
	}
	body := &cppir.Block{Inline: true, Stmts: pre}
	body.Stmts = append(body.Stmts, &cppir.Return{X: list})
	return &cppir.Call{Fun: &cppir.Lambda{Default: "&", Body: body}}
}

// typedFieldValue returns the C++ of the value x of the field f in a designated initializer.
func (fg *funcGen) typedFieldValue(rec *probe.Record, f *probe.Field, x ast.Expr) (cppir.Expr, bool) {
	ft := f.Type.T
	if ft == nil {
		return nil, false
	}
	if ft.Kind == ctype.Array {
		// A C array member: a Go array literal gives its elements, [3]C.int{1, 2, 3} is {1, 2, 3}.
		lit, ok := ast.Unparen(x).(*ast.CompositeLit)
		if !ok {
			return nil, false
		}
		if _, ok := ast.Unparen(lit.Type).(*ast.ArrayType); !ok {
			return nil, false
		}
		elems := make([]cppir.Expr, len(lit.Elts))
		for i, el := range lit.Elts {
			if _, keyed := el.(*ast.KeyValueExpr); keyed {
				return nil, false
			}
			elems[i] = fg.cMoved(el, fg.cValueOf(el))
		}
		return &cppir.InitList{Elems: elems}, true
	}
	cx := fg.cMoved(x, fg.cValueOf(x))
	return fg.typedBrace(rec, f, x, cx), true
}

// typedBrace returns cx, the C++ of x, as the initializer of the scalar field f of rec in a braced
// list: a number of another C++ type is converted first, because narrowing is an error in a braced list
// (-Werror=narrowing; clang's own default) and the compatibility rule (README §2.3) is C++'s
// conversions: gp::cgo::as<T>(x) is Go's T(x), a conversion that is defined for every value.
func (fg *funcGen) typedBrace(rec *probe.Record, f *probe.Field, x ast.Expr, cx cppir.Expr) cppir.Expr {
	g := fg.g
	if f.Type.T == nil {
		return cx
	}
	mapper := g.interop.typed.rep.Mapper
	to := mapper.MapIn(f.Type.T, rec.Scope)
	if to.Kind != ftype.Scalar {
		return cx
	}
	var from types.Type
	if v := g.tvalOf(x); v != nil && v.Kind == ftype.Scalar {
		from = v.Type
	} else if t := g.info.TypeOf(x); t != nil && complete(t) && g.info.Types[x].Value == nil {
		from = t
	}
	if from == nil || sameScalar(from, to.Type) {
		return cx
	}
	return templateCall("gp::cgo::as", []cppir.Node{g.cNode(typeName("decltype(::" + rec.Name + "::" + f.Name + ")"))}, cx)
}

// sameScalar reports whether the numeric types a and b have the same class (integer or floating),
// width and sign, so that a value of one goes to the other unchanged.
func sameScalar(a, b types.Type) bool {
	ba, bb := underBasic(a), underBasic(b)
	if ba == nil || bb == nil {
		return false
	}
	ia, ib := ba.Info(), bb.Info()
	if ia&types.IsNumeric == 0 || ib&types.IsNumeric == 0 {
		return ia&types.IsBoolean != 0 && ib&types.IsBoolean != 0
	}
	if ia&types.IsFloat != ib&types.IsFloat || ia&types.IsComplex != ib&types.IsComplex {
		return false
	}
	return ia&types.IsUnsigned == ib&types.IsUnsigned && scalarBits(ba) == scalarBits(bb)
}

// scalarBits is the size in bits of a basic numeric type (int and uint are 64).
func scalarBits(b *types.Basic) int {
	switch b.Kind() {
	case types.Int8, types.Uint8:
		return 8
	case types.Int16, types.Uint16:
		return 16
	case types.Int32, types.Uint32, types.Float32:
		return 32
	}
	return 64
}

// hasCall reports whether evaluating e calls something that Go orders: a call that is not a
// conversion or one of len and cap, or a receive.
func (g *gen) hasCall(e ast.Expr) bool {
	found := false
	ast.Inspect(e, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.CallExpr:
			if tv, ok := g.info.Types[x.Fun]; ok && tv.IsType() {
				return true
			}
			if g.pkg.Conversion(x) != nil {
				return true
			}
			if id, ok := ast.Unparen(x.Fun).(*ast.Ident); ok {
				if b, ok := g.info.Uses[id].(*types.Builtin); ok && (b.Name() == "len" || b.Name() == "cap") {
					return true
				}
			}
			found = true
		case *ast.UnaryExpr:
			if x.Op.String() == "<-" {
				found = true
			}
		case *ast.FuncLit:
			return false
		}
		return !found
	})
	return found
}
