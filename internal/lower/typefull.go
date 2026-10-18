package lower

import (
	"go/token"
	"go/types"
	"strconv"

	"github.com/huuhait/gocpp/internal/cppir"
)

// The complete descriptors of the named types (Phase 05 step 5.11, ADR 028). The descriptor
// object of a named type with a composite underlying type is constant-initialized with a partial
// shape (typedesc.go: shapeOf), so it can be used before any dynamic initialization. fmt (and
// reflect, Phase 09) need the rest: the field list of a struct with the offsets of the fields,
// the element of a slice, array or pointer, the key and value of a map, the parameters and results
// of a func. They come from a function the descriptor points at (gp::shape_lazy), which makes the
// complete descriptor once, the first time something asks (gp::full):
//
//	inline const gp::type_desc* gp_full_Rect();
//	inline const gp::type_desc gp_td_Rect = gp::named<Rect>("main.Rect", "main",
//	    gp::shape_lazy(gp::shape_struct(nullptr, 0), &gp_full_Rect), ...);
//	...
//	inline const gp::type_desc* gp_full_Rect() {
//	    static const gp::field_desc gp_f[] = {
//	        gp::field_desc("w", "main", &gp::td_float64, nullptr, false, gp::offset_of(&Rect::w)), ...};
//	    static const gp::type_desc gp_d = gp::completed(gp_td_Rect, gp::shape_struct(gp_f, 2));
//	    return &gp_d;
//	}
//
// The function is defined after the pointers to the composite descriptors it names (gp_tdc_N,
// which are interned at program start), and its object stays the identity of the type: the
// complete copy is only read. A part with a C/C++ type, which go/types has no type for, gets its
// descriptor from the C++ type (gp::cxx_desc, gocpp/fmt.hpp): the type of the member
// (decltype(Rect::f)), or the element of the named type's class (gp::detail::elem_t<Items>).

// fullFunc returns the function gp_full_<name> of the named type nd (see the top), or nil if its
// descriptor is complete: the type is no struct, slice, array, map, pointer or func.
func (g *gen) fullFunc(nd *namedDesc, pos token.Pos) *cppir.Func {
	typ := g.cxxType(nd.ty, pos)
	tname := cppir.TypeString(typ)
	// part is the descriptor of a part of the type: a Go type that is complete, else deduced from
	// the C++ type in cxx.
	part := func(t types.Type, cxx string) cppir.Expr {
		if complete(t) {
			return g.desc(t, pos)
		}
		if cxx == "" {
			return lit("nullptr") // (a C/C++ part of a func type: not described)
		}
		return templateCall("gp::cxx_desc", []cppir.Node{typeName(cxx)})
	}
	var stmts []cppir.Stmt
	var shape cppir.Expr
	switch u := nd.t.Underlying().(type) {
	case *types.Slice:
		shape = call("gp::shape_slice", part(u.Elem(), "gp::detail::elem_t<"+tname+">"))
	case *types.Array:
		shape = call("gp::shape_array", lit(strconv.FormatInt(u.Len(), 10)), part(u.Elem(), "gp::detail::elem_t<"+tname+">"))
	case *types.Map:
		shape = call("gp::shape_map", part(u.Key(), "gp::detail::key_t<"+tname+">"), part(u.Elem(), "gp::detail::elem_t<"+tname+">"))
	case *types.Pointer:
		shape = call("gp::shape_pointer", part(u.Elem(), "gp::detail::elem_t<"+tname+">"))
	case *types.Chan:
		shape = call("gp::shape_chan", chanDir(u), part(u.Elem(), "gp::detail::elem_t<"+tname+">"))
	case *types.Signature:
		list := func(name string, tuple *types.Tuple) (cppir.Expr, cppir.Expr) {
			if tuple.Len() == 0 {
				return lit("nullptr"), lit("0")
			}
			var elems []cppir.Expr
			for i := range tuple.Len() {
				elems = append(elems, part(tuple.At(i).Type(), ""))
			}
			stmts = append(stmts, &cppir.VarDecl{Specs: []string{"static"},
				Type: &cppir.Const{Elem: &cppir.Ptr{Elem: &cppir.Const{Elem: typeName("gp::type_desc")}}},
				Vars: []cppir.Declarator{{Name: name + "[]", Init: &cppir.InitList{Elems: elems}}}})
			return cxxName(name), lit(strconv.Itoa(tuple.Len()))
		}
		in, nin := list("gp_in", u.Params())
		out, nout := list("gp_out", u.Results())
		shape = call("gp::shape_func", in, nin, out, nout, lit(strconv.FormatBool(u.Variadic())))
	case *types.Struct:
		if nd.si == nil || nd.si.st == nil {
			return nil
		}
		if u.NumFields() == 0 {
			shape = call("gp::shape_struct", lit("nullptr"), lit("0"))
			break
		}
		var fields []cppir.Expr
		for i := range u.NumFields() {
			f := u.Field(i)
			pkg, tag := cppir.Expr(lit("nullptr")), cppir.Expr(lit("nullptr"))
			if !f.Exported() {
				pkg = lit(cppir.CString(pkgPath(f)))
			}
			if u.Tag(i) != "" {
				tag = lit(cppir.CString(u.Tag(i)))
			}
			member := tname + "::" + nd.si.fields[i]
			fields = append(fields, call("gp::field_desc", lit(cppir.CString(f.Name())), pkg,
				part(f.Type(), "decltype("+member+")"), tag, lit(strconv.FormatBool(f.Embedded())),
				call("gp::offset_of", &cppir.Unary{Op: "&", X: cxxName(member)})))
		}
		stmts = append(stmts, &cppir.VarDecl{Specs: []string{"static", "const"}, Type: typeName("gp::field_desc"),
			Vars: []cppir.Declarator{{Name: "gp_f[]", Init: &cppir.InitList{Elems: fields}}}})
		shape = call("gp::shape_struct", cxxName("gp_f"), lit(strconv.Itoa(u.NumFields())))
	default:
		return nil
	}
	stmts = append(stmts,
		&cppir.VarDecl{Specs: []string{"static", "const"}, Type: typeName("gp::type_desc"),
			Vars: []cppir.Declarator{{Name: "gp_d", Init: call("gp::completed", cxxName(nd.name), shape)}}},
		&cppir.Return{X: addrOf("gp_d")})
	return &cppir.Func{
		Specs:  g.fullSpecs(),
		Result: &cppir.Ptr{Elem: &cppir.Const{Elem: typeName("gp::type_desc")}},
		Name:   "gp_full_" + nd.si.name,
		Body:   &cppir.Block{Stmts: stmts},
	}
}

// fullSpecs are the specifiers of gp_full functions: inline in the unit layout, static in the
// split layout (pkg_desc.cpp defines the descriptor objects and these functions; nothing else
// names them).
func (g *gen) fullSpecs() []string {
	if g.split {
		return []string{"static"}
	}
	return []string{"inline"}
}
