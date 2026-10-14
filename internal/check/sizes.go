package check

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"math"
	"slices"
	"strings"
)

// gocppSizes gives go/types the sizes and alignments of gocpp's types, for
// unsafe.Sizeof, unsafe.Alignof and unsafe.Offsetof, and for constant
// expressions that use them (types.Config.Sizes). They are the sizes of the
// C++ types that the types become (Phase 03 README §2.1, divergence D15 in
// docs/spec/language.md), on every target (Linux and Windows, x86-64):
//
//   - The numeric types and bool have Go's sizes on amd64, which are those
//     of the fixed-width C++ types they become (int is int64_t: 8 bytes).
//   - A type that becomes a runtime type has the layout in the table below
//     (runtimeTypes): a pointer (gp::ptr<T>, a std::shared_ptr) is 16
//     bytes, a string 32, a slice 40, a map 16. weak.Pointer[T] is 24 bytes
//     (gp::weak_ptr<T>), whatever its Go declaration would give.
//   - A struct is laid out like the C++ struct that the emitter writes for
//     it (step 3.8): the fields in source order, each at the next multiple
//     of its alignment, and the size rounded up to the struct's alignment
//     (the largest alignment of a field), so tail padding counts. An empty
//     struct is 1 byte (alignment 1), as in C++, where Go's is 0: an object
//     of an empty C++ class has a size of 1. So every type has a size of at
//     least 1, and struct{ a struct{}; b int64 } is 16 bytes (Go: 8). The
//     emitter must not put [[no_unique_address]] on fields, and it must
//     make embedded fields data members, never base classes (Phase 05 §2.6:
//     no C++ inheritance for embedding): the empty-base optimization would
//     give an embedded empty struct 0 bytes.
//   - An array [N]T (gp::array<T, N>, step 3.6) is N times the size of T,
//     with T's alignment. [0]T is 1 byte with alignment 1: gp::array<T, 0>
//     must be an empty struct (std::array<T, 0> is 1 byte in libstdc++, but
//     has T's size and alignment in libc++, so step 3.6 can't just wrap it).
//   - A named type has the layout of its underlying type: a named struct is
//     that struct, and a gp::newtype (type Celsius float64, step 3.9) must be
//     as big as the type it wraps.
//
// runtime/include/gocpp/sizes.hpp, which rt.hpp includes, is generated from
// the same table and holds a static_assert for each runtime type that the
// runtime has, so the C++ can't drift from these numbers. runtime/test/
// layout.cpp checks struct layouts the same way (offsetof); the runtime's
// tests compile both with g++, clang++ and MinGW-w64. TestSizesHeader fails
// when either file is stale; go test ./internal/check -run TestSizesHeader
// -update-sizes writes them.
//
// types.CheckExpr, which checks the arguments of opaque instantiations
// (checkOpaqueArgs) and the expressions that go/types skipped
// (checkSkipped), takes no types.Config and uses Go's sizes (gc on amd64).
// So unsafe.Sizeof, Alignof and Offsetof there would be Go's sizes;
// checkOpaqueArgs and checkSkipped compute them again with gocppSizes, and
// report an error where the two differ (sizeCallError).
type gocppSizes struct{}

// A layout is the size and the alignment of a type, in bytes.
type layout struct {
	size, align int64
}

// The layouts of the types that become runtime types (Phase 03 README
// §2.1), and of the ones that will, as their phases plan them. Every
// layout here is in runtimeTypes, which sizes.hpp is generated from.
var (
	// gp::string: the owner (a std::shared_ptr, null for literals), the
	// data pointer and the length.
	layoutString = layout{32, 8}
	// gp::ptr<T>: a std::shared_ptr<T> (the pointer and the owner's count
	// block). Also unsafe.Pointer (gp::ptr<void>).
	layoutPointer = layout{16, 8}
	// gp::weak_ptr<T>: a std::weak_ptr<T> and the address it was made
	// from (for ==, also after the target is gone).
	layoutWeak = layout{24, 8}
	// gp::slice<T> (step 3.5): the owner, the data pointer, the length and
	// the capacity.
	layoutSlice = layout{40, 8}
	// gp::map<K, V> (step 3.7): a std::shared_ptr to the table.
	layoutMap = layout{16, 8}
	// gp::chan<T> (Phase 08 §2.2): a shared reference to the channel, a
	// std::shared_ptr. A placeholder until Phase 08 builds it.
	layoutChan = layout{16, 8}
	// gp::func<Sig> (step 3.11): a wrapper of std::function, which is 32
	// bytes in libstdc++ (g++, clang++ on Linux, MinGW-w64; libc++ is not
	// checked). A placeholder until 3.11 decides the layout.
	layoutFunc = layout{32, 8}
	// An interface value (Phase 05 §2.3): the data (a std::shared_ptr),
	// the type descriptor and the method table. A placeholder until Phase
	// 05 builds it.
	layoutInterface = layout{32, 8}
	// A C/C++ type in gocpp code (C.int, *stdio.FILE: the invalid type for
	// go/types) has a size that only the C++ compiler knows (Phase 04,
	// Phase 12). gocppSizes must return something, so it counts as a word,
	// as in Go's gc for an unknown type. That is a guess, so no constant may
	// keep it: checkSizeCParts rejects every unsafe.Sizeof, Alignof and
	// Offsetof whose value depends on such a type.
	layoutCWord = layout{8, 8}
)

// basicLayouts are the layouts of the basic types, by kind: Go's on amd64,
// which are the C++ types' (docs/spec/cpp-mapping.md, "Types").
var basicLayouts = map[types.BasicKind]layout{
	types.Bool:       {1, 1},
	types.Int:        {8, 8},
	types.Int8:       {1, 1},
	types.Int16:      {2, 2},
	types.Int32:      {4, 4},
	types.Int64:      {8, 8},
	types.Uint:       {8, 8},
	types.Uint8:      {1, 1},
	types.Uint16:     {2, 2},
	types.Uint32:     {4, 4},
	types.Uint64:     {8, 8},
	types.Uintptr:    {8, 8},
	types.Float32:    {4, 4},
	types.Float64:    {8, 8},
	types.Complex64:  {8, 4},
	types.Complex128: {16, 8},
	types.String:     layoutString,

	types.UnsafePointer: layoutPointer,
}

// A runtimeType is a row of the size table: a Go type, the C++ type that
// it becomes, and their layout.
type runtimeType struct {
	// goType is the Go type, as Go source (TestSizesTable checks that
	// gocppSizes gives it the layout).
	goType string
	// cpp is the C++ type that sizes.hpp checks: a template gets an
	// example argument (gp::ptr<int64_t>).
	cpp    string
	layout layout
	// defined matches (a regexp) the definition of the C++ type in the
	// runtime's headers; "" for a C++ built-in type. TestSizesTable checks
	// that a type the runtime has is not pending, and that a type that
	// isn't pending is there.
	defined string
	// pending is the step that adds the C++ type to the runtime, or "" when
	// the runtime has it. sizes.hpp asserts the layouts of the types that
	// the runtime has, and lists the pending ones in a comment.
	pending string
}

// runtimeTypes is the size table, the one source of the layouts of gocpp's
// types: gocppSizes uses the layouts above, and sizes.hpp is generated from
// this table. Structs have no row: gocppSizes computes them, and
// runtime/test/layout.cpp checks examples; arrays have example rows.
var runtimeTypes = []runtimeType{
	{goType: "bool", cpp: "bool", layout: basicLayouts[types.Bool]},
	{goType: "int8", cpp: "int8_t", layout: basicLayouts[types.Int8]},
	{goType: "int16", cpp: "int16_t", layout: basicLayouts[types.Int16]},
	{goType: "int32", cpp: "int32_t", layout: basicLayouts[types.Int32]},
	{goType: "int64", cpp: "int64_t", layout: basicLayouts[types.Int64]},
	{goType: "int", cpp: "int64_t", layout: basicLayouts[types.Int]},
	{goType: "uint8", cpp: "uint8_t", layout: basicLayouts[types.Uint8]},
	{goType: "uint16", cpp: "uint16_t", layout: basicLayouts[types.Uint16]},
	{goType: "uint32", cpp: "uint32_t", layout: basicLayouts[types.Uint32]},
	{goType: "uint64", cpp: "uint64_t", layout: basicLayouts[types.Uint64]},
	{goType: "uint", cpp: "uint64_t", layout: basicLayouts[types.Uint]},
	{goType: "uintptr", cpp: "uintptr_t", layout: basicLayouts[types.Uintptr]},
	{goType: "float32", cpp: "float", layout: basicLayouts[types.Float32]},
	{goType: "float64", cpp: "double", layout: basicLayouts[types.Float64]},
	// The emitter has no complex numbers yet; when the runtime includes
	// <complex> for them, sizes.hpp asserts these (layout.cpp does now).
	{goType: "complex64", cpp: "std::complex<float>", layout: basicLayouts[types.Complex64],
		defined: `#include <complex>`, pending: "complex numbers"},
	{goType: "complex128", cpp: "std::complex<double>", layout: basicLayouts[types.Complex128],
		defined: `#include <complex>`, pending: "complex numbers"},
	{goType: "string", cpp: "gp::string", layout: layoutString, defined: `class string\b`},
	{goType: "*int", cpp: "gp::ptr<int64_t>", layout: layoutPointer, defined: `class ptr\b`},
	{goType: "unsafe.Pointer", cpp: "gp::ptr<void>", layout: layoutPointer, defined: `class ptr\b`},
	{goType: "weak.Pointer[int]", cpp: "gp::weak_ptr<int64_t>", layout: layoutWeak,
		defined: `class weak_ptr\b`},
	{goType: "[]int", cpp: "gp::slice<int64_t>", layout: layoutSlice,
		defined: `(?:class|struct) slice\b`, pending: ""},
	// Arrays have no fixed layout (gocppSizes computes them), but these
	// examples make sizes.hpp assert gp::array (step 3.6): an empty struct
	// for N == 0, N elements without gaps otherwise.
	{goType: "[0]int64", cpp: "gp::array<int64_t, 0>", layout: layout{1, 1},
		defined: `(?:class|struct) array\b`, pending: ""},
	{goType: "[3]int16", cpp: "gp::array<int16_t, 3>", layout: layout{6, 2},
		defined: `(?:class|struct) array\b`, pending: ""},
	{goType: "map[int]int", cpp: "gp::map<int64_t, int64_t>", layout: layoutMap,
		defined: `(?:class|struct) map\b`, pending: "step 3.7"},
	{goType: "func()", cpp: "gp::func<void()>", layout: layoutFunc,
		defined: `(?:class|struct) func\b`, pending: "step 3.11"},
	{goType: "any", cpp: "gp::any", layout: layoutInterface,
		defined: `(?:class|struct) any\b`, pending: "Phase 05"},
	{goType: "chan int", cpp: "gp::chan<int64_t>", layout: layoutChan,
		defined: `(?:class|struct) chan\b`, pending: "Phase 08"},
}

func (s gocppSizes) Sizeof(t types.Type) int64 { return s.layoutOf(t).size }

func (s gocppSizes) Alignof(t types.Type) int64 { return s.layoutOf(t).align }

// Offsetsof returns the offsets of the fields of a C++ struct: each field
// at the next multiple of its alignment. An offset is -1 from the field on
// whose offset or size overflows int64 (go/types reports the type as too
// large).
func (s gocppSizes) Offsetsof(fields []*types.Var) []int64 {
	offsets := make([]int64, len(fields))
	var off int64
	for i, f := range fields {
		if off < 0 {
			offsets[i] = -1
			continue
		}
		l := s.layoutOf(f.Type())
		off = alignUp(off, l.align)
		offsets[i] = off
		if l.size < 0 || off > math.MaxInt64-l.size {
			off = -1
		} else {
			off += l.size
		}
	}
	return offsets
}

// layoutOf returns the layout of t. The size is -1 when it overflows int64.
func (s gocppSizes) layoutOf(t types.Type) layout {
	t = types.Unalias(t)
	if n, ok := t.(*types.Named); ok && isWeakPointer(n) {
		return layoutWeak
	}
	switch u := t.Underlying().(type) {
	case *types.Basic:
		if l, ok := basicLayouts[u.Kind()]; ok {
			return l
		}
		return layoutCWord // invalid: a C/C++ type
	case *types.Pointer:
		return layoutPointer
	case *types.Slice:
		return layoutSlice
	case *types.Map:
		return layoutMap
	case *types.Chan:
		return layoutChan
	case *types.Signature:
		return layoutFunc
	case *types.Interface:
		// Also a type parameter, whose underlying type is its constraint:
		// go/types asks only for sizes that don't depend on one.
		return layoutInterface
	case *types.Array:
		return s.arrayLayout(u)
	case *types.Struct:
		return s.structLayout(u)
	}
	return layoutCWord // a tuple or a union: never a value
}

// arrayLayout returns the layout of gp::array<T, N>: N elements without
// gaps (a C++ size is a multiple of the alignment), or an empty struct for
// N == 0.
func (s gocppSizes) arrayLayout(a *types.Array) layout {
	n := a.Len()
	if n <= 0 {
		return layout{1, 1}
	}
	elem := s.layoutOf(a.Elem())
	if elem.size < 0 || elem.size > math.MaxInt64/n {
		return layout{-1, elem.align}
	}
	return layout{elem.size * n, elem.align}
}

// structLayout returns the layout of the C++ struct for st: the end of the
// last field rounded up to the largest alignment of a field, or 1 byte for
// an empty struct.
func (s gocppSizes) structLayout(st *types.Struct) layout {
	n := st.NumFields()
	if n == 0 {
		return layout{1, 1}
	}
	fields := make([]*types.Var, n)
	align := int64(1)
	for i := range n {
		fields[i] = st.Field(i)
		if a := s.layoutOf(fields[i].Type()).align; a > align {
			align = a
		}
	}
	last := s.layoutOf(fields[n-1].Type()).size
	off := s.Offsetsof(fields)[n-1]
	if off < 0 || last < 0 || off > math.MaxInt64-last-(align-1) {
		return layout{-1, align}
	}
	return layout{alignUp(off+last, align), align}
}

// alignUp rounds x up to a multiple of a, a power of 2. It keeps a negative
// x (overflow) negative.
func alignUp(x, a int64) int64 {
	if x < 0 || x > math.MaxInt64-(a-1) {
		return -1
	}
	return (x + a - 1) &^ (a - 1)
}

// isWeakPointer reports whether n is weak.Pointer[T], from the standard
// package weak (gp::weak_ptr<T>, a compiler intrinsic in Phase 03).
func isWeakPointer(n *types.Named) bool {
	obj := n.Origin().Obj()
	return obj.Name() == "Pointer" && obj.Pkg() != nil && obj.Pkg().Path() == "weak"
}

// sizeCallError checks the calls of unsafe.Sizeof, unsafe.Alignof and
// unsafe.Offsetof in e (also through a dot import of unsafe), after
// types.CheckExpr checked e. CheckExpr computes them with Go's sizes, not
// gocpp's (see gocppSizes), so sizeCallError computes each one again with
// gocppSizes. It returns an error for the first one whose value differs
// (where says where the call is: "in a C++ template argument"), or nil:
// unsafe.Sizeof(n) of an int is 8 either way, but unsafe.Sizeof(s) of a
// string is 16 in Go and 32 in gocpp. A constant gets gocpp's value (go/types
// computes it with gocppSizes), so the error says to use one. A call whose
// value depends on a C/C++ type is left to checkSizeCParts, which reports
// it after every check: both numbers would be guesses.
func sizeCallError(fset *token.FileSet, e ast.Expr, info *types.Info, where string) error {
	var err error
	ast.Inspect(e, func(n ast.Node) bool {
		if err != nil {
			return false
		}
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		got, want, ok := sizeCallValues(call, info)
		if ok && got != want {
			src := types.ExprString(call)
			err = types.Error{Fset: fset, Pos: call.Pos(), Msg: fmt.Sprintf(
				"cannot use %s %s: it would be Go's %d, not gocpp's %d (use a constant: const n = %s)",
				src, where, got, want, src)}
		}
		return true
	})
	return err
}

// sizeCall returns the name of the builtin that call calls ("Sizeof",
// "Alignof" or "Offsetof") if it is a call of one of them, through the
// package name unsafe or a dot import, that go/types gave a constant value,
// and that value; else "".
func sizeCall(call *ast.CallExpr, info *types.Info) (name string, value int64) {
	if len(call.Args) != 1 {
		return "", 0
	}
	var id *ast.Ident
	switch fun := ast.Unparen(call.Fun).(type) {
	case *ast.Ident:
		id = fun
	case *ast.SelectorExpr:
		id = fun.Sel
	default:
		return "", 0
	}
	b, isBuiltin := info.Uses[id].(*types.Builtin)
	tv, recorded := info.Types[call]
	if !isBuiltin || b.Pkg() != types.Unsafe || !recorded || tv.Value == nil {
		return "", 0
	}
	switch b.Name() {
	case "Sizeof", "Alignof", "Offsetof":
		v, exact := constant.Int64Val(tv.Value)
		if exact {
			return b.Name(), v
		}
	}
	return "", 0
}

// sizeCallValues returns the value that go/types recorded for call, if it
// is a call of unsafe.Sizeof, Alignof or Offsetof with a constant value
// that doesn't depend on a C/C++ type, and the value with gocppSizes.
func sizeCallValues(call *ast.CallExpr, info *types.Info) (got, want int64, ok bool) {
	name, got := sizeCall(call, info)
	if name == "" {
		return 0, 0, false
	}
	if _, isC := sizeCallCPart(call, name, info); isC {
		return 0, 0, false // checkSizeCParts reports it
	}
	arg := ast.Unparen(call.Args[0])
	var s gocppSizes
	switch name {
	case "Sizeof", "Alignof":
		t := info.TypeOf(arg)
		if t == nil {
			return 0, 0, false
		}
		if name == "Sizeof" {
			return got, s.Sizeof(t), true
		}
		return got, s.Alignof(t), true
	}
	// Offsetof: the offset in the struct of sel.X, through the embedded
	// fields of the path, as go/types computes it.
	st, path, isField := offsetofPath(arg, info)
	if !isField {
		return 0, 0, false
	}
	var off int64
	for k, i := range path {
		fields := structFields(st)
		off += s.Offsetsof(fields)[i]
		if k < len(path)-1 {
			next, isStruct := fields[i].Type().Underlying().(*types.Struct)
			if !isStruct {
				return 0, 0, false
			}
			st = next
		}
	}
	return got, off, true
}

// offsetofPath returns the struct of x and the path of field indexes to f
// (through embedded fields) for the argument x.f of unsafe.Offsetof. x may
// be a pointer to the struct; go/types allows no pointer on the path.
func offsetofPath(arg ast.Expr, info *types.Info) (*types.Struct, []int, bool) {
	sel, isSel := arg.(*ast.SelectorExpr)
	if !isSel {
		return nil, nil, false
	}
	selection := info.Selections[sel]
	if selection == nil || selection.Kind() != types.FieldVal {
		return nil, nil, false
	}
	t := selection.Recv()
	if ptr, isPtr := t.Underlying().(*types.Pointer); isPtr {
		t = ptr.Elem() // unsafe.Offsetof(p.f), p a pointer
	}
	st, isStruct := t.Underlying().(*types.Struct)
	return st, selection.Index(), isStruct
}

// structFields returns the fields of st.
func structFields(st *types.Struct) []*types.Var {
	fields := make([]*types.Var, st.NumFields())
	for i := range fields {
		fields[i] = st.Field(i)
	}
	return fields
}

// sizeCallCPart reports whether the value of call, a call of the unsafe
// builtin name with a constant value, depends on a C/C++ type, and returns
// the fields through which it does (see cPart). Sizeof(x) and Alignof(x)
// depend on all of x's type; Offsetof(x.f) on the fields before f on its
// path and on f's own type (its alignment).
func sizeCallCPart(call *ast.CallExpr, name string, info *types.Info) ([]*types.Var, bool) {
	arg := ast.Unparen(call.Args[0])
	if name != "Offsetof" {
		t := info.TypeOf(arg)
		if t == nil {
			return nil, false
		}
		return cPart(t, nil)
	}
	st, path, isField := offsetofPath(arg, info)
	if !isField {
		return nil, false
	}
	var on []*types.Var // the embedded fields on the way
	for k, i := range path {
		fields := structFields(st)
		for _, f := range fields[:i+1] {
			if part, isC := cPart(f.Type(), nil); isC {
				return slices.Concat(on, []*types.Var{f}, part), true
			}
		}
		if k < len(path)-1 {
			next, isStruct := fields[i].Type().Underlying().(*types.Struct)
			if !isStruct {
				break
			}
			on = append(on, fields[i])
			st = next
		}
	}
	return nil, false
}

// cPart reports whether t holds a C/C++ type by value (the invalid type for
// go/types: C.int, and *stdio.FILE too), through struct fields and array
// elements, but not through Go pointers, slices, maps and the other runtime
// types, whose layout doesn't depend on what they point to. It returns the
// fields on the way to it, outermost first (none when t is an array of a
// C/C++ type). gocppSizes counts such a type as 8 bytes, a guess, so
// checkSizeCParts rejects unsafe.Sizeof, Alignof and Offsetof that depend on
// one. seen guards against cycles (only an invalid program has one by value).
func cPart(t types.Type, seen map[*types.Named]bool) ([]*types.Var, bool) {
	t = types.Unalias(t)
	if n, ok := t.(*types.Named); ok {
		if isWeakPointer(n) || seen[n] {
			return nil, false
		}
		if seen == nil {
			seen = make(map[*types.Named]bool)
		}
		seen[n] = true
	}
	switch u := t.Underlying().(type) {
	case *types.Basic:
		return nil, u.Kind() == types.Invalid
	case *types.Array:
		return cPart(u.Elem(), seen)
	case *types.Struct:
		for i := range u.NumFields() {
			f := u.Field(i)
			if part, isC := cPart(f.Type(), seen); isC {
				return append([]*types.Var{f}, part...), true
			}
		}
	}
	return nil, false
}

// checkSizeCParts reports each call of unsafe.Sizeof, Alignof and Offsetof
// in files whose value depends on a C/C++ type (cPart): the C++ compiler
// knows its size, gocpp's type checker doesn't (Phase 04 brings it). It runs
// after go/types, checkOpaqueArgs and checkSkipped, so it also sees the
// calls that types.CheckExpr checked.
// cppNames maps the place of each name that the header and namespace
// rewrite replaced by C (load.CppUse.Ident) to the name the user wrote, so
// the message shows *stdio.FILE, not *C.FILE.
func checkSizeCParts(fset *token.FileSet, files []*ast.File, info *types.Info, cppNames map[token.Pos]string,
	report func(pos token.Pos, err error)) {
	// The type expressions of the fields declared in files, for the message.
	fieldTypes := make(map[*types.Var]ast.Expr)
	for _, f := range files {
		ast.Inspect(f, func(n ast.Node) bool {
			if fl, ok := n.(*ast.Field); ok {
				for _, id := range fl.Names {
					if v, ok := info.Defs[id].(*types.Var); ok {
						fieldTypes[v] = fl.Type
					}
				}
			}
			return true
		})
	}
	for _, f := range files {
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			name, _ := sizeCall(call, info)
			if name == "" {
				return true
			}
			fields, isC := sizeCallCPart(call, name, info)
			if !isC {
				return true
			}
			part := "its element type" // an array of a C/C++ type
			if len(fields) > 0 {
				names := make([]string, len(fields))
				for i, fv := range fields {
					names[i] = fv.Name()
				}
				part = "field " + strings.Join(names, ".")
				if te, ok := fieldTypes[fields[len(fields)-1]]; ok {
					part += " " + types.ExprString(sourceExpr(te, cppNames))
				}
			}
			report(call.Pos(), types.Error{Fset: fset, Pos: call.Pos(), Msg: fmt.Sprintf(
				"cannot use %s: its type has a C/C++ part (%s), whose size gocpp only knows from Phase 04",
				types.ExprString(call), part)})
			return true
		})
	}
}

// sourceExpr returns e as the user wrote it, undoing the header and
// namespace rewrite (cppNames, see checkSizeCParts): C.FILE was stdio.FILE,
// C.std.vector was std.vector. It rebuilds only the nodes of type
// expressions that can hold such a selector; e itself is not changed.
func sourceExpr(e ast.Expr, cppNames map[token.Pos]string) ast.Expr {
	switch x := e.(type) {
	case *ast.SelectorExpr:
		if c, ok := x.X.(*ast.Ident); ok && c.Name == "C" {
			if name, ok := cppNames[c.Pos()]; ok {
				if x.Sel.Pos() == c.Pos() {
					return x.Sel // C.std was std
				}
				return &ast.SelectorExpr{X: ast.NewIdent(name), Sel: x.Sel} // C.FILE was stdio.FILE
			}
		}
		return &ast.SelectorExpr{X: sourceExpr(x.X, cppNames), Sel: x.Sel}
	case *ast.StarExpr:
		return &ast.StarExpr{X: sourceExpr(x.X, cppNames)}
	case *ast.ParenExpr:
		return &ast.ParenExpr{X: sourceExpr(x.X, cppNames)}
	case *ast.ArrayType:
		return &ast.ArrayType{Len: x.Len, Elt: sourceExpr(x.Elt, cppNames)}
	case *ast.IndexExpr:
		return &ast.IndexExpr{X: sourceExpr(x.X, cppNames), Index: sourceExpr(x.Index, cppNames)}
	case *ast.IndexListExpr:
		indices := make([]ast.Expr, len(x.Indices))
		for i, ix := range x.Indices {
			indices[i] = sourceExpr(ix, cppNames)
		}
		return &ast.IndexListExpr{X: sourceExpr(x.X, cppNames), Indices: indices}
	}
	return e
}
