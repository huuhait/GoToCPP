package check

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"unsafe"
	"weak"
)

// The files that TestGCSizesHeader generates (phase-16 README §2.2 and §2.8; job 16.1e). The
// runtime's tests compile them, so their static_asserts prove the C++ layouts against Go's
// numbers: gcSizesHeader by runtime/test/gc_values_test.cpp and runtime/test/gc_layout_test.cpp,
// and gcLayoutTest by runtime/layout_test.go (g++ and clang++).
const (
	gcSizesHeader = "../../runtime/include/gocpp/gc/sizes.hpp"
	gcLayoutTest  = "../../runtime/test/gc_layout_test.cpp"
)

// gcSizes returns Go's own sizes on amd64: the algorithm and the numbers of gc, which is what
// the GC engine's values must have. (The reference-counting engine's gocppSizes in sizes.go is
// unrelated and unchanged.)
func gcSizes() types.Sizes {
	return types.SizesFor("gc", "amd64")
}

// TestGCSizes checks the simple layouts in gcRuntimeTypes against real Go's unsafe.Sizeof and
// Alignof. The struct shapes are TestGCLayoutShapes's.
func TestGCSizes(t *testing.T) {
	if runtime.GOARCH != "amd64" {
		t.Skipf("the GC engine's sizes are gc's amd64 sizes; this is %s", runtime.GOARCH)
	}
	var iface interface{ M() }
	cases := []struct {
		goType string
		size   uintptr
		align  uintptr
	}{
		{"bool", unsafe.Sizeof(false), unsafe.Alignof(false)},
		{"int8", unsafe.Sizeof(int8(0)), unsafe.Alignof(int8(0))},
		{"int16", unsafe.Sizeof(int16(0)), unsafe.Alignof(int16(0))},
		{"int32", unsafe.Sizeof(int32(0)), unsafe.Alignof(int32(0))},
		{"int64", unsafe.Sizeof(int64(0)), unsafe.Alignof(int64(0))},
		{"int", unsafe.Sizeof(int(0)), unsafe.Alignof(int(0))},
		{"uint8", unsafe.Sizeof(uint8(0)), unsafe.Alignof(uint8(0))},
		{"uint16", unsafe.Sizeof(uint16(0)), unsafe.Alignof(uint16(0))},
		{"uint32", unsafe.Sizeof(uint32(0)), unsafe.Alignof(uint32(0))},
		{"uint64", unsafe.Sizeof(uint64(0)), unsafe.Alignof(uint64(0))},
		{"uint", unsafe.Sizeof(uint(0)), unsafe.Alignof(uint(0))},
		{"uintptr", unsafe.Sizeof(uintptr(0)), unsafe.Alignof(uintptr(0))},
		{"float32", unsafe.Sizeof(float32(0)), unsafe.Alignof(float32(0))},
		{"float64", unsafe.Sizeof(float64(0)), unsafe.Alignof(float64(0))},
		{"string", unsafe.Sizeof(""), unsafe.Alignof("")},
		{"*int", unsafe.Sizeof((*int)(nil)), unsafe.Alignof((*int)(nil))},
		{"unsafe.Pointer", unsafe.Sizeof(unsafe.Pointer(nil)), unsafe.Alignof(unsafe.Pointer(nil))},
		{"[]int", unsafe.Sizeof([]int(nil)), unsafe.Alignof([]int(nil))},
		{"[0]int64", unsafe.Sizeof([0]int64{}), unsafe.Alignof([0]int64{})},
		{"[3]int16", unsafe.Sizeof([3]int16{}), unsafe.Alignof([3]int16{})},
		{"map[int]int", unsafe.Sizeof(map[int]int(nil)), unsafe.Alignof(map[int]int(nil))},
		{"func()", unsafe.Sizeof(func() {}), unsafe.Alignof(func() {})},
		{"any", unsafe.Sizeof(any(nil)), unsafe.Alignof(any(nil))},
		{"interface{ M() }", unsafe.Sizeof(iface), unsafe.Alignof(iface)},
		{"chan int", unsafe.Sizeof(chan int(nil)), unsafe.Alignof(chan int(nil))},
		{"struct{}", unsafe.Sizeof(struct{}{}), unsafe.Alignof(struct{}{})},
	}
	byType := make(map[string]gcRuntimeType, len(gcRuntimeTypes))
	for _, r := range gcRuntimeTypes {
		byType[r.goType] = r
	}
	for _, c := range cases {
		r, ok := byType[c.goType]
		if !ok {
			t.Errorf("gcRuntimeTypes has no row for %s", c.goType)
			continue
		}
		if r.layout.size != int64(c.size) || r.layout.align != int64(c.align) {
			t.Errorf("%s: the table says %d/%d, real Go says %d/%d", c.goType,
				r.layout.size, r.layout.align, c.size, c.align)
		}
	}
}

// TestGCSizesHeader checks that runtime/include/gocpp/gc/sizes.hpp and
// runtime/test/gc_layout_test.cpp are what the GC table and Go's sizes give. -update-sizes
// writes them.
func TestGCSizesHeader(t *testing.T) {
	for _, f := range []struct {
		path string
		text []byte
	}{
		{gcSizesHeader, gcSizesHPP(t)},
		{gcLayoutTest, gcLayoutCPP(t)},
	} {
		if *updateSizes {
			if err := os.WriteFile(f.path, f.text, 0o644); err != nil {
				t.Fatal(err)
			}
			continue
		}
		have, err := os.ReadFile(f.path)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(have, f.text) {
			t.Errorf("%s is stale: run go test ./internal/check -run TestGCSizesHeader -update-sizes", f.path)
		}
	}
}

// formatCXX formats generated C++ with clang-format 21 and the repo's .clang-format, so the
// generated files pass TestClangFormat (which walks runtime/, generated files included).
// TestClangFormat skips when clang-format 21 is missing; so does this, and the size table and
// the layout shapes still run.
func formatCXX(t *testing.T, src []byte, name string) []byte {
	t.Helper()
	cf, err := exec.LookPath("clang-format")
	if err != nil {
		t.Skipf("TestGCSizesHeader needs clang-format 21 to format %s: %v", name, err)
	}
	v, err := exec.Command(cf, "--version").Output()
	if err != nil {
		t.Fatalf("clang-format --version: %v", err)
	}
	if !strings.Contains(string(v), "version 21.") {
		t.Skipf("clang-format is %s; the style is checked with version 21",
			strings.TrimSpace(string(v)))
	}
	assume, err := filepath.Abs(name)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(cf, "-style=file", "-assume-filename="+assume)
	cmd.Stdin = bytes.NewReader(src)
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	if err := cmd.Run(); err != nil {
		t.Fatalf("clang-format %s: %v\n%s", name, err, errOut.String())
	}
	return out.Bytes()
}

// gcSizesHPP returns runtime/include/gocpp/gc/sizes.hpp: a static_assert of the size, the
// alignment and the pointer-ness of each C++ type in the table, against Go's numbers. A
// zero-size Go type is 1 byte in C++ (every complete object is), so its row asserts Go's size
// through go_layout and C++'s through sizeof >= 1.
func gcSizesHPP(t *testing.T) []byte {
	t.Helper()
	var b bytes.Buffer
	b.WriteString(`// gocpp/gc/sizes.hpp: the sizes, alignments and pointer-ness of the GC engine's types,
// checked.
//
// GENERATED from the size table in internal/check/gcsizes.go (gcRuntimeTypes): don't edit it,
// change the table and run go test ./internal/check -run TestGCSizesHeader -update-sizes.
//
// These are Go's numbers (phase-16 README §2.2), not the reference-counting engine's
// (gocpp/sizes.hpp): string 16, slice 24, interface 16, func, map and chan 8. go_layout<T>()
// computes them from the C++ type, so these static_asserts keep the C++ in step with the table;
// runtime/test/gc_layout_test.cpp does the same for structs.
#pragma once

#include <gocpp/gc/rt.hpp>

#include <cstdint>

`)
	seen := make(map[string]bool)
	for _, r := range gcRuntimeTypes {
		if seen[r.cpp] {
			continue
		}
		seen[r.cpp] = true
		var names []string
		for _, o := range gcRuntimeTypes {
			if o.cpp == r.cpp {
				names = append(names, o.goType)
			}
		}
		goTypes := strings.Join(names, ", ")
		if r.layout.size == 0 {
			fmt.Fprintf(&b, "static_assert(gp::gc::go_layout<%s>().size == 0, \"%s: Go size 0\");\n",
				r.cpp, goTypes)
			fmt.Fprintf(&b, "static_assert(alignof(%s) == %d, \"%s: alignment %d\");\n",
				r.cpp, r.layout.align, goTypes, r.layout.align)
			fmt.Fprintf(&b, "static_assert(sizeof(%s) >= 1, \"%s: a C++ object is at least 1 byte\");\n",
				r.cpp, goTypes)
			continue
		}
		fmt.Fprintf(&b, "static_assert(sizeof(%s) == %d, \"%s: %d %s\");\n",
			r.cpp, r.layout.size, goTypes, r.layout.size, byteUnit(r.layout.size))
		fmt.Fprintf(&b, "static_assert(alignof(%s) == %d, \"%s: alignment %d\");\n",
			r.cpp, r.layout.align, goTypes, r.layout.align)
		fmt.Fprintf(&b, "static_assert(gp::gc::go_layout<%s>().size == %d, \"%s: Go size\");\n",
			r.cpp, r.layout.size, goTypes)
		fmt.Fprintf(&b, "static_assert(gp::gc::go_layout<%s>().align == %d, \"%s: Go alignment\");\n",
			r.cpp, r.layout.align, goTypes)
		fmt.Fprintf(&b, "static_assert(gp::gc::go_layout<%s>().ptrs == %t, \"%s: Go pointer-ness\");\n",
			r.cpp, r.ptrs, goTypes)
	}
	return formatCXX(t, b.Bytes(), gcSizesHeader)
}

// gcLayoutCPP returns runtime/test/gc_layout_test.cpp: the structs of gcLayoutSrc in C++ with
// the emitter's helpers (zero-size fields as [[no_unique_address]] zero_field members, explicit
// padding, a static_assert of each offset that occupies bytes), laid out by Go's own sizes.
func gcLayoutCPP(t *testing.T) []byte {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "gclayout.go", gcLayoutSrc, 0)
	if err != nil {
		t.Fatal(err)
	}
	conf := types.Config{Sizes: gcSizes()}
	pkg, err := conf.Check("gclayout", fset, []*ast.File{f}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var names []*types.TypeName
	for _, name := range pkg.Scope().Names() {
		tn, ok := pkg.Scope().Lookup(name).(*types.TypeName)
		if !ok {
			continue
		}
		if _, ok := tn.Type().Underlying().(*types.Struct); ok {
			names = append(names, tn)
		}
	}
	slices.SortFunc(names, func(a, b *types.TypeName) int { return int(a.Pos() - b.Pos()) })

	var b bytes.Buffer
	b.WriteString(`// Struct layouts of the GC engine, checked: the C++ structs that the emitter writes for Go
// structs (the fields in source order), with Go's sizes, alignments and offsets.
//
// GENERATED from internal/check/gcsizes.go (gcLayoutSrc): don't edit it, run go test
// ./internal/check -run TestGCSizesHeader -update-sizes. runtime/layout_test.go compiles it with
// g++ and clang++, and TestGCLayoutShapes checks the Go numbers against the real standard
// types. It has no code to run.
#include <gocpp/gc/rt.hpp>
#include <gocpp/gc/sizes.hpp>

#include <cstddef>
#include <cstdint>

namespace gc_layout_test {

`)
	sizes := gcSizes()
	for _, tn := range names {
		st := tn.Type().Underlying().(*types.Struct)
		writeGCStruct(&b, sizes, tn.Name(), st)
	}
	b.WriteString("} // namespace gc_layout_test\n")
	return formatCXX(t, b.Bytes(), gcLayoutTest)
}

// writeGCStruct writes the C++ struct for a Go struct: the fields in source order, explicit
// padding to each Go offset, a zero_field member for a zero-size field, and Go's one byte after
// a trailing zero-size field. Then the static_asserts.
func writeGCStruct(b *bytes.Buffer, sizes types.Sizes, name string, st *types.Struct) {
	fields := make([]*types.Var, st.NumFields())
	for i := range fields {
		fields[i] = st.Field(i)
	}
	offsets := sizes.Offsetsof(fields)
	var cur int64
	var fieldExprs []string
	fmt.Fprintf(b, "struct %s {\n", name)
	for i, f := range fields {
		off := offsets[i]
		size := sizes.Sizeof(f.Type())
		align := sizes.Alignof(f.Type())
		if off > cur {
			fmt.Fprintf(b, "    gp::gc::padding<%d> gp_pad%d;\n", off-cur, i)
			cur = off
		}
		if size == 0 {
			// The field's Go type is gone from the C++ member, so its comparability is kept
			// explicitly: [0]func() makes log/slog.Value uncomparable.
			cmp := types.Comparable(f.Type())
			switch {
			case align == 1 && cmp:
				fmt.Fprintf(b, "    [[no_unique_address]] gp::gc::zero_field<%d> %s;\n", i, f.Name())
			case cmp:
				fmt.Fprintf(b, "    [[no_unique_address]] gp::gc::zero_field<%d, %d> %s;\n",
					i, align, f.Name())
			default:
				fmt.Fprintf(b, "    [[no_unique_address]] gp::gc::zero_field<%d, %d, false> %s;\n",
					i, align, f.Name())
			}
		} else {
			fmt.Fprintf(b, "    %s %s;\n", gcCxxType(f.Type()), f.Name())
			cur = off + size
		}
		fieldExprs = append(fieldExprs,
			fmt.Sprintf("gp::gc::field(&%s::%s, %q)", name, f.Name(), f.Name()))
	}
	if len(fields) > 0 {
		last := fields[len(fields)-1]
		if sizes.Sizeof(last.Type()) == 0 && sizes.Sizeof(st) > 0 {
			fmt.Fprintf(b, "    gp::gc::padding<1> gp_tail;\n")
		}
	}
	fmt.Fprintf(b, "    static constexpr auto gp_fields = gp::gc::fields(%s);\n",
		strings.Join(fieldExprs, ", "))
	fmt.Fprintf(b, "};\n")
	fmt.Fprintf(b, "GP_GC_LAYOUT(%s, %d, %d);\n", name, sizes.Sizeof(st), sizes.Alignof(st))
	fmt.Fprintf(b, "static_assert(gp::gc::detail::is_comparable_v<%s> == %t, \"%s: Go comparability\");\n",
		name, types.Comparable(st), name)
	for i, f := range fields {
		if sizes.Sizeof(f.Type()) != 0 {
			fmt.Fprintf(b, "GP_GC_FIELD_AT(%s, %s, %d);\n", name, f.Name(), offsets[i])
		}
	}
	b.WriteString("\n")
}

// gcCxxType returns the C++ type that the emitter writes for a Go type in the layout test.
func gcCxxType(t types.Type) string {
	t = types.Unalias(t)
	switch u := t.(type) {
	case *types.Basic:
		switch u.Kind() {
		case types.Bool:
			return "bool"
		case types.Int8:
			return "int8_t"
		case types.Int16:
			return "int16_t"
		case types.Int32:
			return "int32_t"
		case types.Int64, types.Int:
			return "int64_t"
		case types.Uint8:
			return "uint8_t"
		case types.Uint16:
			return "uint16_t"
		case types.Uint32:
			return "uint32_t"
		case types.Uint64, types.Uint:
			return "uint64_t"
		case types.Uintptr:
			return "uintptr_t"
		case types.Float32:
			return "float"
		case types.Float64:
			return "double"
		case types.String:
			return "gp::gc::string"
		case types.UnsafePointer:
			return "void*"
		}
	case *types.Pointer:
		return gcCxxType(u.Elem()) + "*"
	case *types.Slice:
		return "gp::gc::slice<" + gcCxxType(u.Elem()) + ">"
	case *types.Array:
		return fmt.Sprintf("gp::gc::array<%s, %d>", gcCxxType(u.Elem()), u.Len())
	case *types.Map:
		return "gp::gc::map<" + gcCxxType(u.Key()) + ", " + gcCxxType(u.Elem()) + ">"
	case *types.Chan:
		return "gp::gc::chan<" + gcCxxType(u.Elem()) + ">"
	case *types.Signature:
		var params []string
		for i := 0; i < u.Params().Len(); i++ {
			params = append(params, gcCxxType(u.Params().At(i).Type()))
		}
		ret := "void"
		if u.Results().Len() == 1 {
			ret = gcCxxType(u.Results().At(0).Type())
		}
		return "gp::gc::func<" + ret + "(" + strings.Join(params, ", ") + ")>"
	case *types.Interface:
		if u.NumMethods() == 0 {
			return "gp::gc::eface"
		}
		return "gp::gc::iface"
	case *types.Named:
		if _, ok := u.Underlying().(*types.Struct); ok {
			return u.Obj().Name()
		}
		return gcCxxType(u.Underlying())
	}
	panic("gcCxxType: no C++ type for " + t.String())
}

// TestGCLayoutShapes checks the mirror structs of gcLayoutSrc against the real standard types
// and the synthetic shapes with reflect and unsafe: the size, the alignment and every field
// offset by index. Together with gc_layout_test.cpp's static_asserts, this proves that the C++
// helper pattern gives Go's layout.
func TestGCLayoutShapes(t *testing.T) {
	if runtime.GOARCH != "amd64" {
		t.Skipf("the GC engine's layout is gc's amd64 layout; this is %s", runtime.GOARCH)
	}
	type gcEmpty struct{}
	type gcZLast struct {
		A int64
		Z gcEmpty
	}
	type gcZFirst struct {
		Z gcEmpty
		A int64
	}
	type gcZZ struct {
		Z1 gcEmpty
		Z2 gcEmpty
	}
	type gcArrZLast struct {
		A int32
		Z [0]int64
	}
	type gcArrZFirst struct {
		Z [0]int64
		A int32
	}
	type gcArrZMid struct {
		A int32
		Z [0]int64
		B int32
	}
	type gcZeroSizeNamed struct {
		z [0]int64
	}
	type gcNestedZero struct {
		A int32
		Z gcZeroSizeNamed
		B int32
	}

	// mirrors computes the layout of each mirror struct in gcLayoutSrc with Go's sizes.
	mirrors := gcMirrorLayouts(t)
	cases := []struct {
		name string
		real any
	}{
		{"empty", struct{}{}},
		{"zLast", gcZLast{}},
		{"zFirst", gcZFirst{}},
		{"zz", gcZZ{}},
		{"arrZLast", gcArrZLast{}},
		{"arrZFirst", gcArrZFirst{}},
		{"arrZMid", gcArrZMid{}},
		{"nestedZero", gcNestedZero{}},
		{"syncMutex", syncMutexValue()},
		{"syncWaitGroup", syncWaitGroupValue()},
		{"syncRWMutex", syncRWMutexValue()},
		{"syncOnce", syncOnceValue()},
		{"syncPool", syncPoolValue()},
		{"atomicPointer", atomicPointerValue()},
		{"atomicInt64", atomicInt64Value()},
		{"weakPointer", weakPointerValue()},
		{"slogValue", slogValueValue()},
		{"slogAttr", slogAttrValue()},
	}
	for _, c := range cases {
		m, ok := mirrors[c.name]
		if !ok {
			t.Errorf("%s: no mirror struct in gcLayoutSrc", c.name)
			continue
		}
		rt := reflect.TypeOf(c.real)
		if int64(rt.Size()) != m.size || int64(rt.Align()) != m.align {
			t.Errorf("%s: the mirror is %d/%d, real Go is %d/%d", c.name,
				m.size, m.align, rt.Size(), rt.Align())
		}
		if rt.NumField() != len(m.offsets) {
			t.Errorf("%s: the mirror has %d fields, real Go has %d", c.name,
				len(m.offsets), rt.NumField())
			continue
		}
		for i, off := range m.offsets {
			if int64(rt.Field(i).Offset) != off {
				t.Errorf("%s: field %d is at %d in the mirror, at %d in real Go", c.name,
					i, off, rt.Field(i).Offset)
			}
		}
	}
}

// mirrorLayout is the layout of one struct of gcLayoutSrc, from go/types and Go's sizes.
type mirrorLayout struct {
	size    int64
	align   int64
	offsets []int64
}

// gcMirrorLayouts parses gcLayoutSrc and returns the layout of each named struct.
func gcMirrorLayouts(t *testing.T) map[string]mirrorLayout {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "gclayout.go", gcLayoutSrc, 0)
	if err != nil {
		t.Fatal(err)
	}
	pkg, err := (&types.Config{Sizes: gcSizes()}).Check("gclayout", fset, []*ast.File{f}, nil)
	if err != nil {
		t.Fatal(err)
	}
	out := make(map[string]mirrorLayout)
	for _, name := range pkg.Scope().Names() {
		tn, ok := pkg.Scope().Lookup(name).(*types.TypeName)
		if !ok {
			continue
		}
		st, ok := tn.Type().Underlying().(*types.Struct)
		if !ok {
			continue
		}
		fields := make([]*types.Var, st.NumFields())
		for i := range fields {
			fields[i] = st.Field(i)
		}
		out[name] = mirrorLayout{
			size:    gcSizes().Sizeof(tn.Type()),
			align:   gcSizes().Alignof(tn.Type()),
			offsets: gcSizes().Offsetsof(fields),
		}
	}
	return out
}

// The real standard values, in their own file so the imports stay next to their use.
func syncMutexValue() any     { return sync.Mutex{} }
func syncWaitGroupValue() any { return sync.WaitGroup{} }
func syncRWMutexValue() any   { return sync.RWMutex{} }
func syncOnceValue() any      { return sync.Once{} }
func syncPoolValue() any      { return sync.Pool{} }
func atomicPointerValue() any { return atomic.Pointer[int]{} }
func atomicInt64Value() any   { return atomic.Int64{} }
func weakPointerValue() any   { return weak.Pointer[int]{} }
func slogValueValue() any     { return slog.Value{} }
func slogAttrValue() any      { return slog.Attr{} }
