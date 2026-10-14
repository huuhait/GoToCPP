package lower

import (
	"go/ast"
	"go/token"
	"go/types"
	"strconv"

	"github.com/huuhait/gocpp/internal/check"
	"github.com/huuhait/gocpp/internal/cppir"
)

// Named types that aren't structs (Phase 03 README §2.4, step 3.9; the runtime is
// gocpp/newtype.hpp), and the conversions between types that are not identical:
//
//	Go                          C++
//	type Celsius float64        struct Celsius : gp::newtype<Celsius, double> {
//	                                using gp::newtype<Celsius, double>::newtype;
//	                                static constexpr const char* gp_type_name = "main.Celsius";
//	                            };
//	                            static_assert(sizeof(Celsius) == 8 && alignof(Celsius) == 8);
//	type IDs []int              struct IDs : gp::newtype<IDs, gp::slice<int64_t>> {...}: an IDs
//	                            is a gp::slice<int64_t> (its base class)
//	var c Celsius, c = 5        Celsius c{};  c = Celsius(5.0);  (a constant has its type)
//	c*9/5 + 32                  c * Celsius(9.0) / Celsius(5.0) + Celsius(32.0)  (newtype's operators)
//	n / m, n << k (integers)    gp::div(n, m), gp::shl(n, k)  (newtype.hpp's overloads)
//	Celsius(f), float64(c)      Celsius(f), c.gp_value()
//	int(c), Fahrenheit(c)       gp::f2i<int64_t>(c.gp_value()), Fahrenheit(c.gp_value())
//	append(ids, 1), ids[1:]     IDs(gp::append(ids, 1)), IDs(gp::sub(ids, 1))  (the result's type)
//	IDs{1, 2}, make(IDs, n)     IDs(gp::slice_of<int64_t>({1, 2})), IDs(gp::make_slice<int64_t>(n))
//	n + m (type Name string)    Name(n + m)
//	b && c (type ok bool)       ok(b && c)  (a comparison typed ok too: ok(x < y))
//	switch c (an integer type)  switch (c.gp_value()) { case 1: ... }  (the folded constants)
//	var ids IDs = []int{1}      IDs ids = IDs(gp::slice_of<int64_t>({1}));  (implicit: below)
//	var s []int = ids           gp::slice<int64_t> s = ids;  (C++'s derived-to-base copy)
//	var p Point = struct{...}{} Point p = gp::convert<Point>(gp_anon_...{});  (field by field)
//	B(a) (type B A, structs)    gp::convert<B>(a)
//	(*B)(pa)                    gp::ptr_cast<B>(pa)
//	buf[:] (type Buf [8]byte)   gp::sub(gp::base_ptr(buf), 0)  (buf boxed: a gp::ptr<Buf>)
//
// Which named types: a defined type over a basic type (not complex), a string, a slice, an array,
// a map or a pointer, also one declared in a function (moved to the namespace like a local struct,
// typedecl.go). Over a struct it is a struct of its own (typedecl.go); over a func type, an
// interface or a channel it comes with those types (steps 3.11, Phase 05, Phase 08). Its methods
// (step 3.10, methods.go) are members of its struct, as a struct's are.
//
// Every operator of the underlying type works on the named type and gives it (go/types gives the
// result its type): the runtime's gp::newtype has the operators of the numbers (converting back
// to U, so a uint8 wraps), and for a class type its base class has the rest (s[i], gp::len,
// range, gp::get). Where the runtime gives the underlying type (a new slice, string or map),
// the emitter converts the result to the named type: wrapNamed. Where the named type's value
// must be the underlying type (a conversion, a runtime template that deduces a number type: an
// index, a bound, a shift count, a switch tag), it is x.gp_value(): valueOf.
//
// Implicit conversions: Go assigns a value to a variable (or passes it, returns it, stores it in
// a composite literal or as a map key, compares it with ==, or as a switch case) of another
// type when both have identical underlying types and one of them is not a named type
// (assignability). go/types records no conversion there, so the emitter finds those places
// first (noteConversions, for each function and package initializer) and fg.expr converts the
// value (convertTo): to a named type explicitly (IDs(s)), between structs with gp::convert; from
// a named type to its unnamed underlying type C++ copies the base class itself. One value with
// several results is converted element by element where it is used (bind for a, b = f(),
// tupleReturn for return f(), tupleArgs for g(f())), and a range target with = where it is
// assigned (rangeIter).

// isNewtype reports whether t is a named type that becomes a gp::newtype: a defined type whose
// underlying type is not a struct (through aliases).
func isNewtype(t types.Type) bool {
	n, ok := types.Unalias(t).(*types.Named)
	return ok && n.Obj().Pkg() != nil && !isStruct(n)
}

// underBasic returns the basic type under t (through names and aliases: Celsius gives float64),
// or nil if t is no basic type that cxxType knows.
func underBasic(t types.Type) *types.Basic {
	if t == nil {
		return nil
	}
	return basicOf(t.Underlying())
}

// valueOf returns x, the C++ of a value of type t, as its underlying type's value: x.gp_value()
// for a named type, else x.
func valueOf(t types.Type, x cppir.Expr) cppir.Expr {
	if !isNewtype(t) {
		return x
	}
	return &cppir.Call{Fun: &cppir.Member{X: x, Name: "gp_value"}}
}

// wrapNamed returns x, the C++ of a value of t's underlying type, as a value of t: T(x) for a
// named type t, else x.
func (g *gen) wrapNamed(t types.Type, x cppir.Expr, pos token.Pos) cppir.Expr {
	if !isNewtype(t) {
		return x
	}
	return construct(g.cxxType(t, pos), x)
}

// newtypeOK reports whether the named type named (declared by spec) becomes a gp::newtype, or
// reports why not.
func (g *gen) newtypeOK(spec *ast.TypeSpec, named *types.Named) bool {
	switch u := named.Underlying().(type) {
	case *types.Basic:
		if basicTypes[u.Kind()] != "" {
			return true
		}
		g.unsupported(spec.Pos(), "type "+u.Name())
	case *types.Slice, *types.Array, *types.Map, *types.Pointer:
		return true
	case *types.Signature:
		g.unsupported(spec.Pos(), "named func types (Phase 03 step 3.11)")
	case *types.Interface:
		g.unsupported(spec.Pos(), "interface types (Phase 05)")
	case *types.Chan:
		g.unsupported(spec.Pos(), "channel types (Phase 08)")
	default:
		g.unsupported(spec.Pos(), "the named type "+named.Obj().Name())
	}
	return false
}

// addNewtype adds the C++ struct of the named type t (not a struct), which the C++ calls name,
// and the anonymous structs that its underlying type uses.
func (g *gen) addNewtype(t *types.Named, name, goName string) *structInfo {
	si := &structInfo{t: t, under: t.Underlying(), name: name, goName: goName}
	td := g.tdecls()
	td.list = append(td.list, si)
	g.anonStructsIn(si.under)
	return si
}

// newtypeDef returns the definition of the named type si (see the top) and its static_assert:
// the base gp::newtype<T, U>, its constructors, the fmt shim's gp_type_name and, for an
// underlying type with int64, uint64 or uintptr in it, gp_value_tag (gocpp/fmtshim.hpp).
func (g *gen) newtypeDef(si *structInfo) []cppir.Decl {
	// In the struct's body, the names of its base classes (their injected class names) hide
	// the package's types of those names: a type named array is gpkg_main::array there; so do
	// its methods (methods.go).
	hidden := make(map[string]bool)
	for n := range baseNames {
		hidden[n] = true
	}
	for _, m := range g.methodNames(si) {
		hidden[m] = true
	}
	defer g.hide(hidden)()
	base := &cppir.TypeName{Name: "gp::newtype", Args: []cppir.Node{typeName(si.name), g.cxxType(si.under, token.NoPos)}}
	s := &cppir.Struct{Name: si.name, Bases: []cppir.Type{base}}
	// The base's constructors, by its full name: a plain newtype::newtype would name the type
	// itself if it is called newtype (or, from step 3.10, a method named newtype).
	s.Members = append(s.Members, &cppir.Using{Name: cppir.TypeString(base) + "::newtype"})
	if tag := fmtTag(si.under); tag != "" {
		s.Members = append(s.Members, &cppir.Using{Name: "gp_value_tag", Type: typeName(tag)})
	}
	cstr := &cppir.Ptr{Elem: &cppir.Const{Elem: typeName("char")}}
	s.Members = append(s.Members, &cppir.VarDecl{Specs: []string{"static", "constexpr"}, Type: cstr,
		Vars: []cppir.Declarator{{Name: "gp_type_name", Init: lit(cppir.CString(si.goName))}}})
	s.Members = append(s.Members, g.methodDecls(si)...) // methods.go
	return []cppir.Decl{s, g.layoutAssert(si)}
}

// baseNames are the names that a gp::newtype's base classes bring into the body of a named type's
// struct (newtypeDef): gp::newtype itself and the runtime classes it can be over
// (detail::runtime_class). A package's type of one of these names is written qualified there.
var baseNames = map[string]bool{"newtype": true, "string": true, "slice": true, "array": true, "map": true, "ptr": true}

// layoutAssert returns the static_assert of the size and the alignment of the type si that
// gocppSizes computes (internal/check/sizes.go).
func (g *gen) layoutAssert(si *structInfo) cppir.Decl {
	sizes := check.Sizes()
	size := &cppir.Binary{Op: "==", X: call("sizeof", cxxName(si.name)), Y: lit(strconv.FormatInt(sizes.Sizeof(si.t), 10))}
	align := &cppir.Binary{Op: "==", X: call("alignof", cxxName(si.name)), Y: lit(strconv.FormatInt(sizes.Alignof(si.t), 10))}
	return &cppir.StaticAssert{Cond: &cppir.Binary{Op: "&&", X: size, Y: align}}
}

// arrayPtrValue returns p, the C++ of a pointer to an array of type t (gp::ptr<A>), as a pointer
// to a gp::array, which gp::sub, gp::elem_ptr, gp::range_array and gp::len take: gp::base_ptr(p)
// for a pointer to a named array type (type Buf [8]byte), else p.
func arrayPtrValue(t types.Type, p cppir.Expr) cppir.Expr {
	if pt, ok := under(t).(*types.Pointer); ok && isNewtype(pt.Elem()) {
		return call("gp::base_ptr", p)
	}
	return p
}

// largeNamedBox returns the C++ that makes the box of a new variable of the named array type
// typ larger than 128 KiB (largeBox, arrays.go) from value, a literal or a conversion of its
// gp::array converted to typ (Big(gp::array_keyed<int64_t, 32768>({...}))), built in the new
// block as that gp::array, which a Big is (its base class, of the same size):
// gp::ptr_cast<Big>(gp::new_array_keyed<int64_t, 32768>({...})). It returns nil for any other
// value.
func largeNamedBox(typ cppir.Type, value cppir.Expr) cppir.Expr {
	c, ok := value.(*cppir.Call)
	if !ok || len(c.Args) != 1 {
		return nil
	}
	if n, ok := c.Fun.(*cppir.Name); !ok || len(n.Args) != 0 || n.Name != cppir.TypeString(typ) {
		return nil
	}
	var array cppir.Type // the gp::array of the literal
	switch x := c.Args[0].(type) {
	case *cppir.InitList:
		if tn, ok := x.Type.(*cppir.TypeName); ok && tn.Name == "gp::array" {
			array = tn
		}
	case *cppir.Call:
		if n, ok := x.Fun.(*cppir.Name); ok && n.Name == "gp::array_keyed" {
			array = &cppir.TypeName{Name: "gp::array", Args: n.Args}
		}
	}
	if array == nil {
		return nil
	}
	return templateCall("gp::ptr_cast", []cppir.Node{typ}, largeBox(array, c.Args[0]))
}

// needsConversion reports whether a value of type from that Go assigns to the type to needs a
// C++ conversion (convertTo): the types are not identical but their underlying types are, and
// to is a named type that isn't a struct (from is unnamed: []int to IDs), or both are struct
// types (named and anonymous). A named type to its unnamed underlying type needs none: C++
// copies the base class.
func needsConversion(from, to types.Type) bool {
	if from == nil || to == nil || types.Identical(from, to) || !types.Identical(under(from), under(to)) {
		return false
	}
	return isNewtype(to) && !isNewtype(from) || isStruct(from) && isStruct(to)
}

// convertTo returns x, the C++ of a value of type from, as a value of the type to, for Go's
// implicit conversions (needsConversion): T(x) to a named type, gp::convert<T>(x) between two
// struct types, or x.
func (g *gen) convertTo(from, to types.Type, x cppir.Expr, pos token.Pos) cppir.Expr {
	if !needsConversion(from, to) {
		return x
	}
	if isStruct(to) {
		return g.convertStruct(from, to, x, pos)
	}
	return construct(g.cxxType(to, pos), x)
}

// convertStruct returns x, the C++ of a struct of type from, as a struct of type to:
// gp::convert<T>(x), which copies it field by field, or x itself if both have the same C++ type
// (an alias).
func (g *gen) convertStruct(from, to types.Type, x cppir.Expr, pos token.Pos) cppir.Expr {
	typ := g.cxxType(to, pos)
	if cppir.TypeString(typ) == cppir.TypeString(g.cxxType(from, pos)) {
		return x
	}
	if !convertible(from, to) {
		qual := types.RelativeTo(g.pkg.Types)
		g.unsupported(pos, "the conversion from "+types.TypeString(from, qual)+" to "+types.TypeString(to, qual)+
			", whose fields differ in struct tags inside a slice or a map")
		return placeholder
	}
	return templateCall("gp::convert", []cppir.Node{typ}, x)
}

// convertible reports whether gp::convert converts a value of type from to the type to, whose
// underlying types are identical ignoring struct tags: the same type, two struct types whose
// fields are convertible, two pointers (ptr_cast: the base types have the same layout), or two
// arrays of convertible elements. A slice, a map (or a func or a chan) of structs that differ in
// tags is not: Go shares its elements, which C++ would have to reinterpret.
func convertible(from, to types.Type) bool {
	if types.Identical(from, to) {
		return true
	}
	switch f := under(from).(type) {
	case *types.Struct:
		t, ok := under(to).(*types.Struct)
		if !ok || t.NumFields() != f.NumFields() {
			return false
		}
		for i := range f.NumFields() {
			if !convertible(f.Field(i).Type(), t.Field(i).Type()) {
				return false
			}
		}
		return true
	case *types.Pointer:
		_, ok := under(to).(*types.Pointer)
		return ok
	case *types.Array:
		t, ok := under(to).(*types.Array)
		return ok && convertible(f.Elem(), t.Elem())
	}
	return false
}

// implicitConv returns x, the C++ of the expression e, converted to the type that Go converts
// it to implicitly there (noteConversions), if it needs a conversion.
func (fg *funcGen) implicitConv(e ast.Expr, x cppir.Expr) cppir.Expr {
	if to, ok := fg.g.implicit[e]; ok {
		return fg.g.convertTo(fg.g.info.TypeOf(e), to, x, e.Pos())
	}
	return x
}

// noteConversion records that Go converts the value e implicitly to the type to, if that needs a
// C++ conversion (fg.expr makes it: implicitConv).
func (g *gen) noteConversion(e ast.Expr, to types.Type) {
	if !needsConversion(g.info.TypeOf(e), to) {
		return
	}
	if g.implicit == nil {
		g.implicit = make(map[ast.Expr]types.Type)
	}
	g.implicit[e] = to
}

// rangeTypes returns the types of the key and the value of a range over a value of type t: a
// slice, an array (or a pointer to one), a string or a map; nil for the others.
func rangeTypes(t types.Type) (key, value types.Type) {
	switch u := under(t).(type) {
	case *types.Slice:
		return types.Typ[types.Int], u.Elem()
	case *types.Array:
		return types.Typ[types.Int], u.Elem()
	case *types.Pointer:
		if a, ok := under(u.Elem()).(*types.Array); ok {
			return types.Typ[types.Int], a.Elem()
		}
	case *types.Map:
		return u.Key(), u.Elem()
	case *types.Basic:
		if u.Info()&types.IsString != 0 {
			return types.Typ[types.Int], types.Typ[types.Rune]
		}
	}
	return nil, nil
}

// tupleConvs returns, for a value of the result list tuple that Go gives to the types to (the
// parameters of a call or the results of a return), whether any element needs a conversion.
func tupleConvs(tuple *types.Tuple, to *types.Tuple) bool {
	for i := range min(tuple.Len(), to.Len()) {
		if needsConversion(tuple.At(i).Type(), to.At(i).Type()) {
			return true
		}
	}
	return false
}

// tupleReturn returns the statements of return f() whose results Go converts implicitly to the
// function's result types (step 3.9): the results in a temporary, then each one converted,
//
//	auto gp_t0 = f(); return {IDs(std::get<0>(gp_t0)), std::get<1>(gp_t0)};
//
// the return last; or nil if e needs no conversion.
func (fg *funcGen) tupleReturn(p cppir.Pos, e ast.Expr) []cppir.Stmt {
	tuple, ok := fg.g.info.TypeOf(e).(*types.Tuple)
	res := fg.fn.obj.Signature().Results()
	if !ok || tuple.Len() < 2 || !tupleConvs(tuple, res) {
		return nil
	}
	t := fg.names.temp("t")
	elems := make([]cppir.Expr, tuple.Len())
	for i := range elems {
		get := &cppir.Call{Fun: &cppir.Name{Name: "std::get", Args: []cppir.Node{lit(strconv.Itoa(i))}}, Args: []cppir.Expr{cxxName(t)}}
		elems[i] = fg.g.convertTo(tuple.At(i).Type(), res.At(i).Type(), get, e.Pos())
	}
	return []cppir.Stmt{varDecl(p, typeName("auto"), t, fg.expr(e)), &cppir.Return{Pos: p, X: &cppir.InitList{Elems: elems}}}
}

// tupleArgs returns the callable that std::apply calls with the results of g in f(g()), where
// Go converts some of them implicitly to f's parameter types (step 3.9): a lambda that converts
// them, [](auto&& gp_e0, auto&& gp_e1) { return f(IDs(gp_e0), gp_e1); }; or nil if none needs a
// conversion. callee is the C++ name of f.
func (fg *funcGen) tupleArgs(c *ast.CallExpr, callee string) cppir.Expr {
	tuple, ok := fg.g.info.TypeOf(c.Args[0]).(*types.Tuple)
	sig, ok2 := under(fg.g.info.TypeOf(c.Fun)).(*types.Signature)
	if !ok || !ok2 || !tupleConvs(tuple, sig.Params()) {
		return nil
	}
	params := make([]cppir.Param, tuple.Len())
	args := make([]cppir.Expr, tuple.Len())
	ts := make([]types.Type, tuple.Len())
	for i := range params {
		name := "gp_e" + strconv.Itoa(i) // (not gp_a: a deferred call's lambda has those)
		params[i] = cppir.Param{Type: &cppir.Ref{Elem: typeName("auto"), RValue: true}, Name: name}
		args[i] = fg.g.convertTo(tuple.At(i).Type(), sig.Params().At(i).Type(), cxxName(name), c.Pos())
		ts[i] = sig.Params().At(i).Type()
	}
	body := &cppir.Return{X: call(fg.g.funcCallName(callee, ts), args...)}
	return &cppir.Lambda{Params: params, Body: &cppir.Block{Stmts: []cppir.Stmt{body}, Inline: true}}
}

// noteConversions finds the implicit conversions in n (see the top) and records the values that
// need one (noteConversion): an assigned value (=, :=, var x T = v), a return value, an argument,
// an element of a composite literal, a map key, the operand of == and != that is compared with a
// value of another type, and a case value of a switch. sig is the signature of the function n is
// in (nil for the initializers of package variables). One value with several results is
// converted where its elements are used (bind, tupleReturn, tupleArgs), not here.
func (g *gen) noteConversions(n ast.Node, sig *types.Signature) {
	note := func(e ast.Expr, to types.Type) { g.noteConversion(e, to) }
	// values records the values of a list for the types: one value for each type. One value
	// with several results (a, b = f(), return f(), g(f())) is converted element by element
	// where it is used (bind, tupleReturn, tupleArgs).
	values := func(list []ast.Expr, typeOf func(i int) types.Type, n int) {
		if len(list) == 1 && n > 1 {
			return
		}
		for i, e := range list {
			if i < n {
				note(e, typeOf(i))
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
							note(kv.Value, u.Field(k).Type())
						}
					} else if i < u.NumFields() {
						note(elt, u.Field(i).Type())
					}
				case *types.Map:
					if keyed {
						note(kv.Key, u.Key())
						note(kv.Value, u.Elem())
					}
				case *types.Slice:
					if keyed {
						elt = kv.Value
					}
					note(elt, u.Elem())
				case *types.Array:
					if keyed {
						elt = kv.Value
					}
					note(elt, u.Elem())
				}
			}
		case *ast.IndexExpr:
			if m, ok := under(g.info.TypeOf(x.X)).(*types.Map); ok {
				note(x.Index, m.Key())
			}
		case *ast.BinaryExpr:
			if x.Op == token.EQL || x.Op == token.NEQ {
				note(x.Y, g.info.TypeOf(x.X))
			}
		case *ast.SwitchStmt:
			// The tag is compared with each case value, as by ==.
			if x.Tag != nil {
				for _, c := range x.Body.List {
					for _, e := range c.(*ast.CaseClause).List {
						note(e, g.info.TypeOf(x.Tag))
					}
				}
			}
		}
		return true
	})
}

// namedConversion returns the C++ of the conversion T(x) where T or the type of x is a named type
// that isn't a struct (conversion, ops.go): x as its underlying value (valueOf), converted to T's
// underlying type as between unnamed types, and that as a T (wrapNamed). T(nil) is T(), and T(x)
// of a T is x.
func (fg *funcGen) namedConversion(c *ast.CallExpr, to, from types.Type) cppir.Expr {
	arg := c.Args[0]
	switch {
	case fg.g.info.Types[arg].IsNil():
		return construct(fg.g.cxxType(to, c.Pos()))
	case types.Identical(to, from):
		// T(x) of a T: x itself (go/types gives an untyped operand T's type: ok(x > 0)).
		return fg.expr(arg)
	}
	x := valueOf(from, fg.expr(arg))
	uto, ufrom := to, from
	if isNewtype(to) {
		uto = to.Underlying()
	}
	if isNewtype(from) {
		ufrom = from.Underlying()
	}
	return fg.g.wrapNamed(to, fg.convertValue(c, uto, ufrom, x), c.Pos())
}
