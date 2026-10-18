package lower

import (
	"go/token"
	"go/types"

	"github.com/huuhait/gocpp/internal/cppir"
)

// The C++ spelling of gocpp types (docs/spec/cpp-mapping.md, "Types").
//
// Phase 02 has the basic types and the result lists of functions:
//
//	bool                     bool
//	int, int8 ... int64      int64_t, int8_t ... int64_t (int is 64 bits: gocpp has only 64-bit targets)
//	uint, uint8 ... uint64   uint64_t, uint8_t ... uint64_t
//	uintptr                  uintptr_t
//	byte, rune               uint8_t, int32_t (aliases of uint8 and int32, in Go too)
//	float32, float64         float, double
//	string                   gp::string
//	(A, B) (results)         std::tuple<A, B>
//	*T                       gp::ptr<T> (Phase 03 step 3.3; T of any type that cxxType has)
//	unsafe.Pointer           gp::ptr<void> (Phase 04 step 4.10, unsafe.go)
//	[]T                      gp::slice<T> (Phase 03 step 3.5; T of any type that cxxType has)
//	[N]T                     gp::array<T, N> (Phase 03 step 3.6, arrays.go)
//	map[K]V                  gp::map<K, V> (Phase 03 step 3.7, maps.go)
//	struct{...}, type T struct{...}   a struct of its own (Phase 03 step 3.8, typedecl.go): Point,
//	                         or gpkg_main::Point where a variable of that name hides it;
//	                         gp_anon_3f2a... for an anonymous struct type
//	type Celsius float64     Celsius, a struct over gp::newtype<Celsius, double> (Phase 03 step
//	type IDs []int           3.9, named.go), IDs over gp::newtype<IDs, gp::slice<int64_t>>
//	func(A, B) R             gp::func<R(A, B)> (Phase 03 step 3.11, closures.go)
//	complex64, complex128    gp::complex64, gp::complex128 (Phase 03 step 3.15c, complex.go)
//	weak.Pointer[T]          gp::weak_ptr<T> (Phase 03 step 3.18, weak.go)
//	chan T, <-chan T, chan<- T   gp::chan<T> (Phase 08 steps 8.3 and 8.4, chans.go): one C++ type for
//	                         the three directions, and three Go types (typedesc.go)
//
// An untyped constant has the C++ type of its default type (untyped int → int64_t, untyped
// rune → int32_t, untyped float → double, untyped complex → gp::complex128), but go/types
// records the type that a constant converts to, so the emitter seldom needs it.
//
//	any, interface{}         gp::any (Phase 05 step 5.4, ifaces.go)
//	error                    gp::error
//	interface{ M() }         a struct over gp::iface: Shape, or gp_anon_i... for an unnamed type
//

// basicTypes are the C++ spellings of the basic types, by kind.
var basicTypes = map[types.BasicKind]string{
	types.Bool:    "bool",
	types.Int:     "int64_t",
	types.Int8:    "int8_t",
	types.Int16:   "int16_t",
	types.Int32:   "int32_t",
	types.Int64:   "int64_t",
	types.Uint:    "uint64_t",
	types.Uint8:   "uint8_t",
	types.Uint16:  "uint16_t",
	types.Uint32:  "uint32_t",
	types.Uint64:  "uint64_t",
	types.Uintptr: "uintptr_t",
	types.Float32: "float",
	types.Float64: "double",
	types.String:  "gp::string",

	types.Complex64:  "gp::complex64", // Phase 03 step 3.15c, complex.go
	types.Complex128: "gp::complex128",

	types.UntypedBool:    "bool",
	types.UntypedInt:     "int64_t",
	types.UntypedRune:    "int32_t",
	types.UntypedFloat:   "double",
	types.UntypedComplex: "gp::complex128",
	types.UntypedString:  "gp::string",
}

// basicOf returns the basic type of t (through aliases), or nil if t is not a basic type
// that cxxType knows.
func basicOf(t types.Type) *types.Basic {
	b, ok := types.Unalias(t).(*types.Basic)
	if !ok || basicTypes[b.Kind()] == "" {
		return nil
	}
	return b
}

// cxxType returns the C++ spelling of t. For a type that Phase 02 doesn't have, it reports
// "not supported yet" at pos and returns a placeholder.
func (g *gen) cxxType(t types.Type, pos token.Pos) cppir.Type {
	switch t := types.Unalias(t).(type) {
	case *types.Basic:
		if s := basicTypes[t.Kind()]; s != "" {
			return typeName(s)
		}
		if t.Kind() == types.UnsafePointer {
			return typeName("gp::ptr<void>") // an owning pointer (unsafe.go)
		}
	case *types.Pointer:
		return ptrType(g.cxxType(t.Elem(), pos))
	case *types.Slice:
		return sliceType(g.cxxType(t.Elem(), pos))
	case *types.Array:
		return arrayType(g.cxxType(t.Elem(), pos), t.Len())
	case *types.Map:
		if cmpHasC(t.Key()) {
			break // its gp_hash and == are the C++ compiler's (typedecl.go)
		}
		return mapType(g.cxxType(t.Key(), pos), g.cxxType(t.Elem(), pos))
	case *types.Chan:
		return chanType(g.cxxType(t.Elem(), pos)) // chan T, <-chan T, chan<- T: one C++ type (chans.go)
	case *types.Struct:
		return g.structType(t, pos) // typedecl.go
	case *types.Named:
		if isWeakPointer(t) {
			return weakType(g.cxxType(weakElem(t), pos)) // weak.go
		}
		if isInterface(t) {
			return g.ifaceType(t, pos) // ifaces.go
		}
		if isStruct(t) || isNewtype(t) {
			return g.structType(t, pos) // typedecl.go; a gp::newtype: named.go
		}
	case *types.Interface:
		return g.ifaceType(t, pos) // ifaces.go
	case *types.Signature:
		return g.funcType(t, pos) // gp::func<R(P...)> (closures.go)
	case *types.Tuple:
		args := make([]cppir.Node, t.Len())
		for i := range t.Len() {
			args[i] = g.cxxType(t.At(i).Type(), pos)
		}
		return &cppir.TypeName{Name: "std::tuple", Args: args}
	}
	if hasC(t) {
		g.unsupported(pos, "this use of a C/C++ type (a later step of Phase 04)")
		return typeName("gp_unsupported")
	}
	g.unsupported(pos, "type "+types.TypeString(t, types.RelativeTo(g.pkg.Types)))
	return typeName("gp_unsupported")
}

// resultType returns the C++ result type of a function with the signature sig: void, the one
// result's type, or a std::tuple of the results.
func (g *gen) resultType(sig *types.Signature, pos token.Pos) cppir.Type {
	switch r := sig.Results(); r.Len() {
	case 0:
		return typeName("void")
	case 1:
		return g.varType(r.At(0), pos)
	default:
		if !complete(r) {
			args := make([]cppir.Node, r.Len())
			for i := range r.Len() {
				args[i] = g.varType(r.At(i), pos) // C/C++ types (ctypes.go)
			}
			return &cppir.TypeName{Name: "std::tuple", Args: args}
		}
		return g.cxxType(r, pos)
	}
}

// ptrType returns gp::ptr<elem>, the C++ type of a pointer to elem, and of the box of a
// variable of the C++ type elem (box.go).
func ptrType(elem cppir.Type) cppir.Type {
	return &cppir.TypeName{Name: "gp::ptr", Args: []cppir.Node{elem}}
}

// isPointer reports whether t is a pointer type (through names and aliases: type P *int).
func isPointer(t types.Type) bool {
	_, ok := under(t).(*types.Pointer)
	return ok
}

// sliceType returns gp::slice<elem>, the C++ type of a slice of elem.
func sliceType(elem cppir.Type) cppir.Type {
	return &cppir.TypeName{Name: "gp::slice", Args: []cppir.Node{elem}}
}

// isSlice reports whether t is a slice type (through names and aliases: type IDs []int).
func isSlice(t types.Type) bool {
	_, ok := under(t).(*types.Slice)
	return ok
}

// isString reports whether t is a string type (through names and aliases), also an untyped one.
func isString(t types.Type) bool {
	b, ok := under(t).(*types.Basic)
	return ok && b.Info()&types.IsString != 0
}
