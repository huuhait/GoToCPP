package lower

import (
	"go/token"
	"go/types"
	"sort"
	"strconv"

	"github.com/huuhait/gocpp/internal/cppir"
)

// Type descriptors, method tables and thunks (Phase 05 step 5.4; the runtime is gocpp/type.hpp
// and gocpp/iface.hpp, docs/spec/cpp-mapping.md "Interfaces"). Every type that flows into an
// interface (a conversion, an assertion, a type switch case) has a descriptor, chosen by the
// Go type:
//
//	int, string, float64     &gp::td_int, &gp::td_string, &gp::td_float64 (the runtime's; int64 is
//	                         &gp::td_int64: another object, though the C++ type is the same)
//	any, error               &gp::td_any, &gp::td_error
//	type Rect struct{...}    &gp_td_Rect      gp::named<Rect>("main.Rect", "main", shape, vm, pm)
//	type Shape interface     &gp_td_Shape     (ifaces.go)
//	*Rect, []int, [3]T, map  a namespace-level pointer, interned at program start:
//	                         static const gp::type_desc* const gp_tdp_Rect = gp::td_ptr<Rect>(&gp_td_Rect);
//	                         static const gp::type_desc* const gp_tdc_0 = gp::td_slice<int64_t>(&gp::td_int);
//
// A named type's descriptor object has its two method tables (the method set of T and of *T)
// and the thunks they hold:
//
//	inline double gp_thunk_Rect_Area(const gp::iface& self) { return gp::unbox<Rect>(self).Area(); }
//	inline double gp_thunk_pRect_Area(const gp::iface& self) {      // Area through a *Rect
//	    gp::ptr<Rect> p = gp::unbox<gp::ptr<Rect>>(self);
//	    gp::check_wrap(p, "value method main.Rect.Area called using nil *Rect pointer");
//	    return p->Area();
//	}
//	inline void gp_thunk_pRect_Scale(const gp::iface& self, double a0) {
//	    Rect::Scale(gp::unbox<gp::ptr<Rect>>(self), static_cast<double&&>(a0));
//	}
//	inline const gp::method gp_vm_Rect[] = {gp::method_of<double()>("Area", nullptr, "func() float64", &gp_thunk_Rect_Area)};
//	inline const gp::method gp_pm_Rect[] = {...Area, Scale...};
//
// The tables are sorted as the runtime says (canon.go: methodLess), the signatures are the
// canonical texts (canon.go), and an unexported method carries its package's import path. The
// shape of a named type has the kind of its underlying type and, where the element's
// descriptor is a constant (a predeclared or a named type), the element; the other parts are
// null (the field list of a struct too): the descriptor object stays constant-initialized, so
// it can be used before any dynamic initialization, and %T, ==, assertions and the itabs need
// nothing more. The reflection-like uses (step 5.11's fmt, Phase 09) take the rest from the
// C++ types.
//
// The descriptors are made on demand (desc): the registry grows while the functions are
// lowered, and descDecls writes everything last, in dependency order. A promoted method (a
// method of an embedded field, step 5.8) is a forwarding thunk of the embedding step; until then
// a type with one can't be converted to an interface.

type descTable struct {
	named   []*namedDesc
	byNamed map[*types.TypeName]*namedDesc
	comps   []*compDesc
	byKey   map[string]*compDesc
}

// A namedDesc is the descriptor object of a named type that isn't an interface.
type namedDesc struct {
	t    *types.Named
	si   *structInfo
	name string // gp_td_Rect
}

// A compDesc is a namespace-level pointer to the interned descriptor of a composite type.
type compDesc struct {
	name string
	init cppir.Expr
}

func (g *gen) dtable() *descTable {
	if g.descs == nil {
		g.descs = &descTable{byNamed: make(map[*types.TypeName]*namedDesc), byKey: make(map[string]*compDesc)}
	}
	return g.descs
}

// basicDescs are the runtime's descriptors of the predeclared types.
var basicDescs = map[types.BasicKind]string{
	types.Bool: "td_bool", types.Int: "td_int", types.Int8: "td_int8", types.Int16: "td_int16",
	types.Int32: "td_int32", types.Int64: "td_int64", types.Uint: "td_uint", types.Uint8: "td_uint8",
	types.Uint16: "td_uint16", types.Uint32: "td_uint32", types.Uint64: "td_uint64",
	types.Uintptr: "td_uintptr", types.Float32: "td_float32", types.Float64: "td_float64",
	types.Complex64: "td_complex64", types.Complex128: "td_complex128", types.String: "td_string",
	types.UnsafePointer: "td_unsafe_pointer",
}

// basicKinds are gp::kind's names of the predeclared types.
var basicKinds = map[types.BasicKind]string{
	types.Bool: "Bool", types.Int: "Int", types.Int8: "Int8", types.Int16: "Int16", types.Int32: "Int32",
	types.Int64: "Int64", types.Uint: "Uint", types.Uint8: "Uint8", types.Uint16: "Uint16",
	types.Uint32: "Uint32", types.Uint64: "Uint64", types.Uintptr: "Uintptr", types.Float32: "Float32",
	types.Float64: "Float64", types.Complex64: "Complex64", types.Complex128: "Complex128",
	types.String: "String", types.UnsafePointer: "UnsafePointer",
}

// addrOf returns &name.
func addrOf(name string) cppir.Expr { return &cppir.Unary{Op: "&", X: cxxName(name)} }

// desc returns the C++ expression of type const gp::type_desc* for the Go type t: the
// descriptor of the type that Go's interface holds for a value of type t.
func (g *gen) desc(t types.Type, pos token.Pos) cppir.Expr {
	t = types.Unalias(t)
	if t == nil || !complete(t) {
		// (A named type over Go types that has C/C++ parts, such as a struct with a C field, is
		// complete: its descriptor is partial, with no field list, so it needs none of them.)
		g.unsupported(pos, "a C/C++ type in a type descriptor (a C/C++ value is held by gp::foreign_td, not described here)")
		return placeholder
	}
	dt := g.dtable()
	switch u := t.(type) {
	case *types.Basic:
		if d := basicDescs[u.Kind()]; d != "" {
			return addrOf("gp::" + d)
		}
		if u.Info()&types.IsUntyped != 0 {
			return g.desc(types.Default(u), pos)
		}
	case *types.Named:
		switch {
		case isErrorType(u):
			return addrOf("gp::td_error")
		case isInterface(u):
			if info := g.ifaceOf(u, pos); info != nil {
				return addrOf("gp_td_" + info.name)
			}
			return placeholder
		case isWeakPointer(u):
			g.unsupported(pos, "weak.Pointer in an interface")
			return placeholder
		}
		nd := dt.byNamed[u.Obj()]
		if nd == nil {
			si := g.structOf(u, pos)
			if si == nil {
				return placeholder
			}
			nd = &namedDesc{t: u, si: si, name: "gp_td_" + si.name}
			dt.byNamed[u.Obj()] = nd
			dt.named = append(dt.named, nd)
		}
		return addrOf(nd.name)
	case *types.Interface:
		if isAnyType(u) {
			return addrOf("gp::td_any")
		}
		info := g.ifaceOf(u, pos)
		if info == nil {
			return placeholder
		}
		return g.comp(t, "", call(info.name+"::gp_desc"))
	case *types.Pointer:
		elem := g.desc(u.Elem(), pos)
		name := ""
		if n, ok := types.Unalias(u.Elem()).(*types.Named); ok && dt.byNamed[n.Obj()] != nil {
			name = "gp_tdp_" + dt.byNamed[n.Obj()].si.name
		}
		return g.comp(t, name, templateCall("gp::td_ptr", []cppir.Node{g.cxxType(u.Elem(), pos)}, elem))
	case *types.Slice:
		return g.comp(t, "", templateCall("gp::td_slice", []cppir.Node{g.cxxType(u.Elem(), pos)}, g.desc(u.Elem(), pos)))
	case *types.Array:
		return g.comp(t, "", templateCall("gp::td_array", []cppir.Node{g.cxxType(u.Elem(), pos), lit(strconv.FormatInt(u.Len(), 10))}, g.desc(u.Elem(), pos)))
	case *types.Map:
		return g.comp(t, "", templateCall("gp::td_map", []cppir.Node{g.cxxType(u.Key(), pos), g.cxxType(u.Elem(), pos)},
			g.desc(u.Key(), pos), g.desc(u.Elem(), pos)))
	case *types.Signature:
		var in, out []cppir.Expr
		for i := range u.Params().Len() {
			in = append(in, g.desc(u.Params().At(i).Type(), pos))
		}
		for i := range u.Results().Len() {
			out = append(out, g.desc(u.Results().At(i).Type(), pos))
		}
		return g.comp(t, "", templateCall("gp::td_func", []cppir.Node{g.cxxType(u, pos)},
			&cppir.InitList{Elems: in}, &cppir.InitList{Elems: out}, lit(strconv.FormatBool(u.Variadic()))))
	case *types.Struct:
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
			fields = append(fields, call("gp::field_desc", lit(cppir.CString(f.Name())), pkg, g.desc(f.Type(), pos), tag, lit(strconv.FormatBool(f.Embedded()))))
		}
		if hasPromoted(u) {
			g.unsupported(pos, "an anonymous struct with embedded fields in an interface (step 5.8)")
		}
		return g.comp(t, "", templateCall("gp::td_struct", []cppir.Node{g.cxxType(u, pos)}, &cppir.InitList{Elems: fields}))
	case *types.Chan:
		g.unsupported(pos, "channel types (Phase 08)")
		return placeholder
	}
	g.unsupported(pos, "the type descriptor of "+types.TypeString(t, nil))
	return placeholder
}

// hasPromoted reports whether the struct type st has an embedded field.
func hasPromoted(st *types.Struct) bool {
	for i := range st.NumFields() {
		if st.Field(i).Embedded() {
			return true
		}
	}
	return false
}

// comp registers the composite descriptor of t (once, by its identity text) and returns the
// name of its pointer. name may be "" (gp_tdc_N).
func (g *gen) comp(t types.Type, name string, init cppir.Expr) cppir.Expr {
	dt := g.dtable()
	key := g.canonText(t)
	if c := dt.byKey[key]; c != nil {
		return cxxName(c.name)
	}
	if name == "" {
		name = "gp_tdc_" + strconv.Itoa(len(dt.comps))
	}
	c := &compDesc{name: name, init: init}
	dt.byKey[key] = c
	dt.comps = append(dt.comps, c)
	return cxxName(c.name)
}

// constDesc returns the descriptor of t if it is a constant (&gp::td_int, &gp_td_Rect), else nil.
func (g *gen) constDesc(t types.Type, pos token.Pos) cppir.Expr {
	switch u := types.Unalias(t).(type) {
	case *types.Basic:
		if basicDescs[u.Kind()] != "" {
			return g.desc(t, pos)
		}
	case *types.Named:
		if complete(u) && !hasC(u) {
			return g.desc(t, pos)
		}
	case *types.Interface:
		if isAnyType(u) {
			return g.desc(t, pos)
		}
	}
	return nil
}

// shapeOf returns the shape of the underlying type of the named type n (see the top).
func (g *gen) shapeOf(n *types.Named, pos token.Pos) cppir.Expr {
	orNull := func(t types.Type) cppir.Expr {
		if d := g.constDesc(t, pos); d != nil {
			return d
		}
		return lit("nullptr")
	}
	switch u := n.Underlying().(type) {
	case *types.Basic:
		return call("gp::shape_basic", cxxName("gp::kind::"+basicKinds[u.Kind()]))
	case *types.Slice:
		return call("gp::shape_slice", orNull(u.Elem()))
	case *types.Array:
		return call("gp::shape_array", lit(strconv.FormatInt(u.Len(), 10)), orNull(u.Elem()))
	case *types.Map:
		return call("gp::shape_map", orNull(u.Key()), orNull(u.Elem()))
	case *types.Pointer:
		return call("gp::shape_pointer", orNull(u.Elem()))
	case *types.Signature:
		return call("gp::shape_func", lit("nullptr"), lit("0"), lit("nullptr"), lit("0"), lit(strconv.FormatBool(u.Variadic())))
	case *types.Struct:
		return call("gp::shape_struct", lit("nullptr"), lit("0"))
	}
	g.unsupported(pos, "the type "+g.canonName(n)+" in an interface")
	return placeholder
}

// A methodEntry is a method in the method set of a named type or of a pointer to it.
type methodEntry struct {
	fn      *types.Func
	ptrRecv bool // declared with a pointer receiver
}

// methodSet returns the methods of the method set of n (ptr: of *n), sorted as the runtime's
// tables are.
func (g *gen) methodSet(n *types.Named, ptr bool, pos token.Pos) []methodEntry {
	var t types.Type = n
	if ptr {
		t = types.NewPointer(n)
	}
	ms := types.NewMethodSet(t)
	var list []methodEntry
	for i := range ms.Len() {
		sel := ms.At(i)
		fn, ok := sel.Obj().(*types.Func)
		if !ok || fn.Name() == "_" {
			continue
		}
		if !complete(fn.Signature()) {
			// A method with a C/C++ type in its signature can't satisfy an interface (no interface
			// method has one: ifaces.go), so the table leaves it out.
			continue
		}
		if len(sel.Index()) > 1 {
			g.unsupported(pos, "the promoted method "+fn.Name()+" of "+g.canonName(n)+" in an interface (struct embedding: step 5.8)")
			continue
		}
		_, p := types.Unalias(fn.Signature().Recv().Type()).(*types.Pointer)
		list = append(list, methodEntry{fn: fn, ptrRecv: p})
	}
	sort.SliceStable(list, func(i, j int) bool { return methodLess(list[i].fn, list[j].fn) })
	return list
}

// descDecls returns the declarations of all the descriptors (see the top): the descriptor
// objects of the named types with their thunks and method tables, then the pointers to the
// composite descriptors. It runs after the functions are lowered.
func (g *gen) descDecls() []cppir.Decl {
	dt := g.dtable()
	var decls []cppir.Decl
	for i := 0; i < len(dt.named); i++ { // (the shapes may add more)
		decls = append(decls, g.namedDecls(dt.named[i])...)
	}
	for _, c := range dt.comps {
		decls = append(decls, &cppir.VarDecl{
			Specs: []string{"static"},
			Type:  &cppir.Const{Elem: &cppir.Ptr{Elem: &cppir.Const{Elem: typeName("gp::type_desc")}}},
			Vars:  []cppir.Declarator{{Name: c.name, Init: c.init}},
		})
	}
	if len(dt.comps) > 0 {
		decls = append(decls, &cppir.Blank{})
	}
	return decls
}

// namedDecls returns the thunks, the method tables and the descriptor object of a named type.
func (g *gen) namedDecls(nd *namedDesc) []cppir.Decl {
	var decls []cppir.Decl
	pos := nd.t.Obj().Pos()
	typ := g.cxxType(nd.t, pos)
	vm := g.methodSet(nd.t, false, pos)
	pm := g.methodSet(nd.t, true, pos)
	table := func(prefix string, list []methodEntry, ptr bool) cppir.Expr {
		if len(list) == 0 {
			return nil
		}
		var elems []cppir.Expr
		for _, m := range list {
			thunk := g.thunk(nd, m, ptr, &decls)
			sig := m.fn.Signature()
			params, result := g.sigCxx(sig, pos)
			args := []cppir.Expr{lit(cppir.CString(m.fn.Name())), lit("nullptr"), lit(cppir.CString(g.canonText(sig))), addrOf(thunk)}
			if !exportedName(m.fn.Name()) {
				args[1] = lit(cppir.CString(pkgPath(m.fn)))
			}
			maker := "gp::method_of"
			if g.awareFunc(m.fn) {
				maker = "gp::method_of_aware" // the method calls recover: `defer i.M()` gives it the ticket (recover.go)
			}
			elems = append(elems, &cppir.Call{
				Fun:  &cppir.Name{Name: maker, Args: []cppir.Node{&cppir.FuncType{Result: result, Params: params}}},
				Args: args,
			})
		}
		name := prefix + nd.si.name
		decls = append(decls, &cppir.VarDecl{Specs: []string{"inline", "const"}, Type: typeName("gp::method"),
			Vars: []cppir.Declarator{{Name: name + "[]", Init: &cppir.InitList{Elems: elems}}}})
		return call("gp::methods_of", cxxName(name))
	}
	args := []cppir.Expr{
		lit(cppir.CString(g.canonName(nd.t))),
		lit(cppir.CString(importPath(nd.t.Obj().Pkg()))),
		g.shapeOf(nd.t, pos),
	}
	vmt := table("gp_vm_", vm, false)
	pmt := table("gp_pm_", pm, true)
	switch {
	case pmt != nil && vmt != nil:
		args = append(args, vmt, pmt)
	case pmt != nil:
		args = append(args, &cppir.InitList{}, pmt)
	}
	decls = append(decls, &cppir.VarDecl{Specs: []string{"inline", "const"}, Type: typeName("gp::type_desc"),
		Vars: []cppir.Declarator{{Name: nd.name, Init: templateCall("gp::named", []cppir.Node{typ}, args...)}}})
	decls = append(decls, &cppir.Blank{})
	return decls
}

// thunk adds the thunk of the method m of the named type nd for the table of T (ptr false) or of
// *T (ptr true) to decls, and returns its name (see the top).
func (g *gen) thunk(nd *namedDesc, m methodEntry, ptr bool, decls *[]cppir.Decl) string {
	pos := m.fn.Pos()
	name := "gp_thunk_" + nd.si.name + "_" + m.fn.Name()
	if ptr {
		name = "gp_thunk_p" + nd.si.name + "_" + m.fn.Name()
	}
	member := g.methodName(nd.si.name, m.fn.Name())
	typ := g.cxxType(nd.t, pos)
	params, result := g.sigCxx(m.fn.Signature(), pos)
	fn := &cppir.Func{Specs: []string{"inline"}, Result: result, Name: name, Body: &cppir.Block{},
		Params: []cppir.Param{{Type: &cppir.Ref{Elem: &cppir.Const{Elem: typeName("gp::iface")}}, Name: "gp_self"}}}
	var args []cppir.Expr
	for k, p := range params {
		an := "gp_a" + strconv.Itoa(k)
		fn.Params = append(fn.Params, cppir.Param{Type: p, Name: an})
		if _, isRef := p.(*cppir.Ref); isRef {
			args = append(args, cxxName(an))
		} else {
			args = append(args, moved(p, cxxName(an)))
		}
	}
	ptrType := ptrType(typ)
	unboxPtr := templateCall("gp::unbox", []cppir.Node{ptrType}, cxxName("gp_self"))
	var invoke cppir.Expr
	switch {
	case m.ptrRecv:
		// A pointer method: only in the table of *T.
		invoke = &cppir.Call{Fun: cxxName(nd.si.name + "::" + member), Args: append([]cppir.Expr{unboxPtr}, args...)}
	case !ptr:
		val := templateCall("gp::unbox", []cppir.Node{typ}, cxxName("gp_self"))
		invoke = &cppir.Call{Fun: &cppir.Member{X: val, Name: member}, Args: args}
	default:
		// A value method through a *T: Go's wrapper checks for nil.
		fn.Body.Stmts = append(fn.Body.Stmts,
			varDecl(cppir.Pos{}, ptrType, "gp_p", unboxPtr),
			&cppir.ExprStmt{X: call("gp::check_wrap", cxxName("gp_p"), lit(cppir.CString(
				"value method "+g.canonName(nd.t)+"."+m.fn.Name()+" called using nil *"+nd.t.Obj().Name()+" pointer")))})
		invoke = &cppir.Call{Fun: &cppir.Member{X: cxxName("gp_p"), Arrow: true, Name: member}, Args: args}
	}
	if tn, ok := result.(*cppir.TypeName); ok && tn.Name == "void" && len(tn.Args) == 0 {
		fn.Body.Stmts = append(fn.Body.Stmts, &cppir.ExprStmt{X: invoke})
	} else {
		fn.Body.Stmts = append(fn.Body.Stmts, &cppir.Return{X: invoke})
	}
	*decls = append(*decls, fn)
	return name
}
