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
//
// An untyped constant has the C++ type of its default type (untyped int → int64_t, untyped
// rune → int32_t, untyped float → double), but go/types records the type that a constant
// converts to, so the emitter seldom needs it.
//
// Not yet: complex64 and complex128 (std::complex, not in the Phase 02 subset), named types,
// also those over a basic type (type Celsius float64 becomes a gp::newtype in Phase 03), and
// every composite type (pointers, slices, arrays, maps, structs, funcs: Phase 03; interfaces
// and error: Phase 05; channels: Phase 08). For these, cxxType reports "not supported yet".

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

	types.UntypedBool:   "bool",
	types.UntypedInt:    "int64_t",
	types.UntypedRune:   "int32_t",
	types.UntypedFloat:  "double",
	types.UntypedString: "gp::string",
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
	case *types.Tuple:
		args := make([]cppir.Node, t.Len())
		for i := range t.Len() {
			args[i] = g.cxxType(t.At(i).Type(), pos)
		}
		return &cppir.TypeName{Name: "std::tuple", Args: args}
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
		return g.cxxType(r.At(0).Type(), pos)
	default:
		return g.cxxType(r, pos)
	}
}
