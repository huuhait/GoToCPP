package check

import (
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/constant"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/load"
)

var updateSizes = flag.Bool("update-sizes", false,
	"rewrite runtime/include/gocpp/sizes.hpp and runtime/test/layout.cpp from the size table")

// The files that TestSizesHeader generates, and the runtime's headers.
const (
	sizesHeader = "../../runtime/include/gocpp/sizes.hpp"
	layoutTest  = "../../runtime/test/layout.cpp"
	runtimeHdrs = "../../runtime/include/gocpp"
)

// weakSrc is a stand-in for the standard package weak, with Go's
// declaration of Pointer: gocppSizes must not use it (it would give 16).
const weakSrc = `package weak

import "unsafe"

type Pointer[T any] struct {
	_ [0]*T
	u unsafe.Pointer
}

func Make[T any](ptr *T) Pointer[T] { return Pointer[T]{} }
`

// sizesImporter imports unsafe and the stand-in weak package (weakSrc).
type sizesImporter struct {
	t    *testing.T
	weak *types.Package
}

func (im *sizesImporter) Import(path string) (*types.Package, error) {
	switch path {
	case "unsafe":
		return types.Unsafe, nil
	case "weak":
		if im.weak == nil {
			im.weak = checkSizesSrc(im.t, "weak.go", weakSrc, im, nil)
		}
		return im.weak, nil
	}
	return nil, fmt.Errorf("no package %s in this test", path)
}

// checkSizesSrc type-checks src with gocppSizes, plainly with go/types
// (no gocpp loader: the cases need only unsafe and weak), and fails the
// test on an error. info may be nil.
func checkSizesSrc(t *testing.T, name, src string, im types.Importer, info *types.Info) *types.Package {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, name, src, 0)
	if err != nil {
		t.Fatal(err)
	}
	conf := types.Config{GoVersion: GoVersion, Importer: im, Sizes: gocppSizes{}}
	pkg, err := conf.Check(strings.TrimSuffix(name, ".go"), fset, []*ast.File{f}, info)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return pkg
}

// TestSizes checks unsafe.Sizeof, Alignof and Offsetof with gocppSizes for
// every kind of type, against the C++ layouts by hand: the basic types,
// the runtime types, weak.Pointer (24 bytes, not what its declaration
// gives), empty structs and zero-length arrays (1 byte, as in C++), padding
// between fields and at the end, nested structs and arrays, embedded
// fields, named types and aliases.
func TestSizes(t *testing.T) {
	cases := []struct {
		expr string
		want int64
	}{
		// The basic types: Go's sizes on amd64.
		{"unsafe.Sizeof(false)", 1},
		{"unsafe.Sizeof(int8(0))", 1},
		{"unsafe.Sizeof(int16(0))", 2},
		{"unsafe.Sizeof(int32(0))", 4},
		{"unsafe.Sizeof(int64(0))", 8},
		{"unsafe.Sizeof(int(0))", 8},
		{"unsafe.Sizeof(uint(0))", 8},
		{"unsafe.Sizeof(uintptr(0))", 8},
		{"unsafe.Sizeof(rune(0))", 4},
		{"unsafe.Sizeof(byte(0))", 1},
		{"unsafe.Sizeof(float32(0))", 4},
		{"unsafe.Sizeof(1.5)", 8}, // float64
		{"unsafe.Sizeof(complex64(0))", 8},
		{"unsafe.Alignof(complex64(0))", 4},
		{"unsafe.Sizeof(complex128(0))", 16},
		{"unsafe.Alignof(complex128(0))", 8},
		{"unsafe.Alignof(int16(0))", 2},
		{"unsafe.Alignof(uint32(0))", 4},
		{"unsafe.Alignof(false)", 1},

		// The runtime types (and the placeholders of the later phases).
		{`unsafe.Sizeof("hi")`, 32},
		{"unsafe.Alignof(str)", 8},
		{"unsafe.Sizeof(&n)", 16},
		{"unsafe.Alignof(&n)", 8},
		{"unsafe.Sizeof(unsafe.Pointer(nil))", 16},
		{"unsafe.Sizeof(wp)", 24},
		{"unsafe.Alignof(wp)", 8},
		{"unsafe.Sizeof(weak.Make(&n))", 24},
		{"unsafe.Sizeof(wpp)", 24},
		{"unsafe.Sizeof(xs)", 40},
		{"unsafe.Alignof(xs)", 8},
		{"unsafe.Sizeof(m)", 16},
		{"unsafe.Sizeof(ch)", 16},
		{"unsafe.Sizeof(fn)", 24},
		{"unsafe.Sizeof(main)", 24},
		{"unsafe.Sizeof(a)", 32},
		{"unsafe.Sizeof(err)", 32},
		{"unsafe.Alignof(err)", 8},

		// Empty structs and zero-length arrays: 1 byte, alignment 1.
		{"unsafe.Sizeof(struct{}{})", 1},
		{"unsafe.Alignof(struct{}{})", 1},
		{"unsafe.Sizeof(E{})", 1},
		{"unsafe.Sizeof([0]int64{})", 1},
		{"unsafe.Alignof([0]int64{})", 1},
		{"unsafe.Sizeof([0]string{})", 1},
		{"unsafe.Sizeof([3]struct{}{})", 3},
		{"unsafe.Sizeof(emptyFirst{})", 16},
		{"unsafe.Offsetof(emptyFirst{}.b)", 8},
		{"unsafe.Sizeof(emptyLast{})", 16},
		{"unsafe.Offsetof(emptyLast{}.e)", 8},
		{"unsafe.Sizeof(twoEmpty{})", 2},
		{"unsafe.Offsetof(twoEmpty{}.b)", 1},
		{"unsafe.Sizeof(zeroArr{})", 8},
		{"unsafe.Offsetof(zeroArr{}.u)", 4},

		// Arrays: N elements, the element's alignment.
		{"unsafe.Sizeof([3]int8{})", 3},
		{"unsafe.Sizeof([2]string{})", 64},
		{"unsafe.Alignof([2]string{})", 8},
		{"unsafe.Sizeof([2]small{})", 12},
		{"unsafe.Alignof([2]small{})", 2},
		{"unsafe.Sizeof([2][3]int16{})", 12},
		{"unsafe.Sizeof([4]*int{})", 64},

		// Structs: each field at the next multiple of its alignment, the
		// size rounded up to the largest alignment.
		{"unsafe.Sizeof(small{})", 6},
		{"unsafe.Alignof(small{})", 2},
		{"unsafe.Offsetof(small{}.b)", 2},
		{"unsafe.Offsetof(small{}.c)", 4},
		{"unsafe.Sizeof(mixed{})", 64},
		{"unsafe.Offsetof(mixed{}.s)", 8},
		{"unsafe.Offsetof(mixed{}.p)", 40},
		{"unsafe.Offsetof(mixed{}.b)", 56},
		{"unsafe.Sizeof(tail{})", 40},
		{"unsafe.Offsetof(tail{}.b)", 32},
		{"unsafe.Sizeof(nested{})", 144},
		{"unsafe.Offsetof(nested{}.in)", 8},
		{"unsafe.Offsetof(nested{}.e)", 72},
		{"unsafe.Offsetof(nested{}.t)", 80},
		{"unsafe.Offsetof(nested{}.f)", 120},
		{"unsafe.Offsetof(nested{}.arr)", 124},
		{"unsafe.Alignof(nested{})", 8},
		{"unsafe.Sizeof(withWeak{})", 80},
		{"unsafe.Offsetof(withWeak{}.w)", 8},
		{"unsafe.Offsetof(withWeak{}.u)", 32},
		{"unsafe.Offsetof(withWeak{}.c)", 48},
		{"unsafe.Offsetof(withWeak{}.d)", 56},
		{"unsafe.Offsetof(withWeak{}.x)", 72},

		// Embedded fields: Offsetof through the path, also through a
		// pointer to the struct.
		{"unsafe.Sizeof(embeds{})", 48},
		{"unsafe.Offsetof(embeds{}.tail)", 8},
		{"unsafe.Offsetof(embeds{}.b)", 40},
		{"unsafe.Offsetof(pe.b)", 40},

		// Named types and aliases: the underlying type's layout.
		{"unsafe.Sizeof(celsius(0))", 8},
		{"unsafe.Sizeof(ids(nil))", 40},
		{"unsafe.Sizeof(mixedAlias{})", 64},
		{"unsafe.Sizeof(named(nil))", 16},
	}
	var b strings.Builder
	b.WriteString(`package main

import (
	"unsafe"
	"weak"
)

type E struct{}
type small struct {
	a int8
	b int16
	c int8
}
type mixed struct {
	a int8
	s string
	p *int
	b bool
}
type tail struct {
	s string
	b int8
}
type nested struct {
	x   int8
	in  mixed
	e   E
	t   tail
	f   float32
	arr [3]small
}
type withWeak struct {
	b bool
	w weak.Pointer[mixed]
	u unsafe.Pointer
	c complex64
	d complex128
	x int8
}
type emptyFirst struct {
	e struct{}
	b int64
}
type emptyLast struct {
	b int64
	e E
}
type twoEmpty struct {
	a, b E
}
type zeroArr struct {
	z [0]string
	u uint32
}
type embeds struct {
	x int8
	tail
}
type celsius float64
type ids []int
type mixedAlias = mixed
type named *mixed

var (
	n   int
	str string
	wp  weak.Pointer[int]
	wpp weak.Pointer[[4]string]
	xs  []int
	m   map[string]int
	ch  chan int
	fn  func(int) string
	a   any
	err error
	pe  *embeds
)

func main() {}

`)
	for i, c := range cases {
		fmt.Fprintf(&b, "const c%d = %s\n", i, c.expr)
	}
	pkg := checkSizesSrc(t, "main.go", b.String(), &sizesImporter{t: t}, nil)
	for i, c := range cases {
		v := pkg.Scope().Lookup(fmt.Sprintf("c%d", i)).(*types.Const).Val()
		if got, ok := constant.Int64Val(v); !ok || got != c.want {
			t.Errorf("%s = %v, want %d", c.expr, v, c.want)
		}
	}
}

// TestSizesTooLarge checks that a type too large for int64 is go/types'
// "too large" error, not an overflowed size.
func TestSizesTooLarge(t *testing.T) {
	for _, src := range []string{
		"var x [1 << 59]string\nconst c = unsafe.Sizeof(x)",
		"var x [1 << 62][4]int8\nconst c = unsafe.Sizeof(x)",
		"var x struct {\n\ta [1 << 58]string\n\tb [1 << 58]string\n}\nconst c = unsafe.Sizeof(x)",
		"var x struct {\n\ta [1 << 62]int8\n\tb [1 << 62]int8\n\tc int8\n}\nconst c = unsafe.Offsetof(x.c)",
	} {
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, "a.go", "package p\n\nimport \"unsafe\"\n\n"+src+"\n", 0)
		if err != nil {
			t.Fatal(err)
		}
		var errs []string
		conf := types.Config{Importer: &sizesImporter{t: t}, Sizes: gocppSizes{},
			Error: func(err error) { errs = append(errs, err.Error()) }}
		_, _ = conf.Check("p", fset, []*ast.File{f}, nil)
		if !slices.ContainsFunc(errs, func(e string) bool { return strings.Contains(e, "too large") }) {
			t.Errorf("%s:\nerrors %q, want one with \"too large\"", src, errs)
		}
	}
}

// TestSizesTable checks the size table (runtimeTypes): gocppSizes gives
// each Go type the row's layout, a row that says the runtime has its C++
// type matches a definition in the runtime's headers, and a pending row
// matches none. So when a step adds a type (gp::slice in step 3.5), this
// test fails until its row is no longer pending and sizes.hpp asserts it.
func TestSizesTable(t *testing.T) {
	var b strings.Builder
	b.WriteString("package main\n\nimport (\n\t\"unsafe\"\n\t\"weak\"\n)\n\n")
	for i, r := range runtimeTypes {
		fmt.Fprintf(&b, "var v%d %s\n", i, r.goType)
	}
	b.WriteString("\nvar _, _ = unsafe.Pointer(nil), weak.Pointer[int]{}\n")
	pkg := checkSizesSrc(t, "table.go", b.String(), &sizesImporter{t: t}, nil)
	var s gocppSizes
	for i, r := range runtimeTypes {
		typ := pkg.Scope().Lookup(fmt.Sprintf("v%d", i)).Type()
		if got := (layout{s.Sizeof(typ), s.Alignof(typ)}); got != r.layout {
			t.Errorf("%s: gocppSizes gives %d bytes, alignment %d; the table says %d, %d",
				r.goType, got.size, got.align, r.layout.size, r.layout.align)
		}
	}
	hdrs := runtimeHeaders(t)
	for _, r := range runtimeTypes {
		if r.defined == "" {
			continue
		}
		file := definingHeader(r, hdrs)
		switch {
		case r.pending == "" && file == "":
			t.Errorf("%s: the table says the runtime has %s, but no header of runtime/include/gocpp matches %q",
				r.goType, r.cpp, r.defined)
		case r.pending != "" && file != "":
			t.Errorf("%s: the runtime has %s now (%s); its row in runtimeTypes (internal/check/sizes.go) is still "+
				"pending (%s): set pending to \"\" and run go test ./internal/check -run TestSizesHeader -update-sizes",
				r.goType, r.cpp, file, r.pending)
		}
	}
}

// runtimeHeaders returns the runtime's headers by name, without sizes.hpp.
func runtimeHeaders(t *testing.T) map[string][]byte {
	t.Helper()
	entries, err := os.ReadDir(runtimeHdrs)
	if err != nil {
		t.Fatal(err)
	}
	hdrs := make(map[string][]byte)
	for _, e := range entries {
		if e.IsDir() || filepath.Base(sizesHeader) == e.Name() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(runtimeHdrs, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		hdrs[e.Name()] = data
	}
	return hdrs
}

// definingHeader returns the name of the first header (by name) that
// matches r.defined, or "".
func definingHeader(r runtimeType, hdrs map[string][]byte) string {
	re := regexp.MustCompile(`(?m)^\s*` + r.defined)
	names := make([]string, 0, len(hdrs))
	for name := range hdrs {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		if re.Match(hdrs[name]) {
			return name
		}
	}
	return ""
}

// TestSizesHeader checks that runtime/include/gocpp/sizes.hpp and
// runtime/test/layout.cpp are what the size table and gocppSizes give.
// -update-sizes writes them. The runtime's tests compile both, with g++,
// clang++ and MinGW-w64, so their static_asserts prove the layouts.
func TestSizesHeader(t *testing.T) {
	for _, f := range []struct {
		path string
		text []byte
	}{
		{sizesHeader, sizesHPP(runtimeHeaders(t))},
		{layoutTest, layoutCPP(t)},
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
			t.Errorf("%s is stale: run go test ./internal/check -run TestSizesHeader -update-sizes", f.path)
		}
	}
}

// sizesHPP returns sizes.hpp: a static_assert of the size and the
// alignment of each C++ type in the table that the runtime has (once for a
// C++ type that several Go types become), and a comment line for each
// pending one. It includes the headers that define the types (hdrs).
func sizesHPP(hdrs map[string][]byte) []byte {
	var b bytes.Buffer
	b.WriteString(`// gocpp/sizes.hpp: the sizes and alignments of gocpp's types, checked.
//
// GENERATED from the size table in internal/check/sizes.go (runtimeTypes): don't edit it, change
// the table and run go test ./internal/check -run TestSizesHeader -update-sizes.
//
// go/types gets these layouts from gocppSizes, so unsafe.Sizeof, Alignof and Offsetof give what
// the C++ gives (divergence D15 in docs/spec/language.md). These static_asserts keep the C++ in
// step with the table. Structs and arrays are computed from them (runtime/test/layout.cpp).
#pragma once

`)
	var incs []string
	for _, r := range runtimeTypes {
		if r.pending == "" && r.defined != "" {
			if h := definingHeader(r, hdrs); h != "" && !slices.Contains(incs, h) {
				incs = append(incs, h)
			}
		}
	}
	for _, h := range incs {
		fmt.Fprintf(&b, "#include <gocpp/%s>\n", h)
	}
	b.WriteString("\n#include <cstdint>\n\n")
	var pending []string
	for i, r := range runtimeTypes {
		if r.pending != "" {
			pending = append(pending, fmt.Sprintf("//   %s, %s: %d %s, alignment %d (%s)",
				r.goType, r.cpp, r.layout.size, byteUnit(r.layout.size), r.layout.align, r.pending))
			continue
		}
		if slices.ContainsFunc(runtimeTypes[:i], func(o runtimeType) bool { return o.cpp == r.cpp }) {
			continue
		}
		var names []string
		for _, o := range runtimeTypes {
			if o.cpp == r.cpp {
				names = append(names, o.goType)
			}
		}
		goTypes := strings.Join(names, ", ")
		fmt.Fprintf(&b, "static_assert(sizeof(%s) == %d, \"%s: %d %s\");\n",
			r.cpp, r.layout.size, goTypes, r.layout.size, byteUnit(r.layout.size))
		fmt.Fprintf(&b, "static_assert(alignof(%s) == %d, \"%s: alignment %d\");\n",
			r.cpp, r.layout.align, goTypes, r.layout.align)
	}
	if len(pending) > 0 {
		b.WriteString("\n// Not in the runtime yet. When a step adds one of these types, TestSizesTable fails until\n" +
			"// its row in the table is no longer pending; then this file asserts it.\n")
		for _, p := range pending {
			b.WriteString(p + "\n")
		}
	}
	return b.Bytes()
}

// byteUnit returns "byte" for 1 and "bytes" for other sizes.
func byteUnit(size int64) string {
	if size == 1 {
		return "byte"
	}
	return "bytes"
}

// layoutSrc holds the structs that runtime/test/layout.cpp checks: C++
// structs with the fields in source order, as the emitter writes them for
// Go structs (Phase 03 §2.4, step 3.8), with the runtime's types as fields.
// Mixed gets default member initializers and a defaulted ==, as the
// emitter writes them (withMembers); neither changes the layout. Arrays,
// slices and maps join when the runtime has them (3.5 to 3.7).
const layoutSrc = `package main

import (
	"unsafe"
	"weak"
)

type Empty struct{}

type Small struct {
	a int8
	b int16
	c int8
}

type Mixed struct {
	a int8
	s string
	p *int
	b bool
}

type Tail struct {
	s string
	b int8
}

type Nested struct {
	x  int8
	in Mixed
	e  Empty
	t  Tail
	f  float32
	sm Small
}

type EmptyFirst struct {
	e Empty
	n int64
}

type EmptyLast struct {
	n int64
	e Empty
}

type Pointers struct {
	b  bool
	w  weak.Pointer[Mixed]
	u  unsafe.Pointer
	pn *Nested
	r  rune
	pp **Small
}

type Complex struct {
	b   bool
	c64 complex64
	x   int8
	c   complex128
	f   float32
}

func main() {}
`

// layoutCPP returns runtime/test/layout.cpp: the structs of layoutSrc in
// C++, with static_asserts of their sizes, alignments and field offsets
// from gocppSizes.
func layoutCPP(t *testing.T) []byte {
	t.Helper()
	info := &types.Info{Defs: make(map[*ast.Ident]types.Object)}
	pkg := checkSizesSrc(t, "layout.go", layoutSrc, &sizesImporter{t: t}, info)
	var b bytes.Buffer
	b.WriteString(`// Struct layouts, checked: the C++ structs that the emitter writes for Go structs (the fields
// in source order), with the sizes, alignments and offsets that go/types gets from gocppSizes.
//
// GENERATED from internal/check/sizes_test.go (layoutSrc): don't edit it, run go test
// ./internal/check -run TestSizesHeader -update-sizes. runtime/layout_test.go compiles it with
// g++ and clang++, and TestWindowsSyntax with MinGW-w64; it has no code to run.
#include <gocpp/rt.hpp>

#include <cstddef>
#include <cstdint>

`)
	b.WriteString("namespace layout_test {\n")
	var s gocppSizes
	// The structs in source order (a struct's fields come before it).
	var names []*types.TypeName
	for id, obj := range info.Defs {
		if tn, ok := obj.(*types.TypeName); ok && id.Pos().IsValid() && tn.Parent() == pkg.Scope() {
			names = append(names, tn)
		}
	}
	slices.SortFunc(names, func(a, b *types.TypeName) int { return int(a.Pos() - b.Pos()) })
	for _, tn := range names {
		st := tn.Type().Underlying().(*types.Struct)
		fmt.Fprintf(&b, "\nstruct %s {", tn.Name())
		if st.NumFields() == 0 {
			b.WriteString("};\n")
		} else {
			b.WriteString("\n")
			for i := range st.NumFields() {
				f := st.Field(i)
				init := ""
				if withMembers[tn.Name()] {
					init = zeroInit(f.Type())
				}
				fmt.Fprintf(&b, "    %s %s%s;\n", layoutCxxType(t, f.Type()), f.Name(), init)
			}
			if withMembers[tn.Name()] {
				fmt.Fprintf(&b, "    bool operator==(const %s&) const = default;\n", tn.Name())
			}
			b.WriteString("};\n")
		}
		fmt.Fprintf(&b, "static_assert(sizeof(%s) == %d && alignof(%s) == %d);\n",
			tn.Name(), s.Sizeof(tn.Type()), tn.Name(), s.Alignof(tn.Type()))
		fields := make([]*types.Var, st.NumFields())
		for i := range fields {
			fields[i] = st.Field(i)
		}
		for i, off := range s.Offsetsof(fields) {
			fmt.Fprintf(&b, "static_assert(offsetof(%s, %s) == %d);\n", tn.Name(), fields[i].Name(), off)
		}
	}
	b.WriteString("\n} // namespace layout_test\n")
	return b.Bytes()
}

// withMembers are the structs of layoutSrc that layout.cpp writes with
// default member initializers and a defaulted ==, like the emitter.
var withMembers = map[string]bool{"Mixed": true}

// zeroInit returns the default member initializer of a field of type typ,
// as the emitter writes it (" = 0" for a number, " = false" for a bool),
// or "" for a class type, whose default constructor makes the zero value.
func zeroInit(typ types.Type) string {
	b, ok := typ.(*types.Basic)
	switch {
	case !ok:
		return ""
	case b.Info()&types.IsBoolean != 0:
		return " = false"
	case b.Info()&types.IsNumeric != 0:
		return " = 0"
	}
	return ""
}

// layoutCxxType returns the C++ type of a field of layoutSrc: a basic
// type (its row in the table), a pointer, a weak pointer or a struct of
// layoutSrc.
func layoutCxxType(t *testing.T, typ types.Type) string {
	t.Helper()
	switch u := typ.(type) {
	case *types.Basic:
		if u.Kind() == types.UnsafePointer {
			return "gp::ptr<void>"
		}
		for _, r := range runtimeTypes {
			if r.goType == types.Typ[u.Kind()].Name() { // int32 for rune
				return r.cpp
			}
		}
	case *types.Pointer:
		return "gp::ptr<" + layoutCxxType(t, u.Elem()) + ">"
	case *types.Named:
		if isWeakPointer(u) {
			return "gp::weak_ptr<" + layoutCxxType(t, u.TypeArgs().At(0)) + ">"
		}
		if _, ok := u.Underlying().(*types.Struct); ok {
			return u.Obj().Name()
		}
	}
	t.Fatalf("layoutSrc: no C++ type for %s", typ)
	return ""
}

// TestSizeCParts checks that unsafe.Sizeof, Alignof and Offsetof whose
// value depends on a C/C++ type held by value (a struct field, an array
// element, also *stdio.FILE, which go/types can't tell from C.int) are
// errors, also in an array length and in a C++ template argument (not
// "Go's 24, not gocpp's 40": both would be guesses). Through a Go pointer,
// slice or map, and before the C/C++ field (Offsetof), the size is known.
func TestSizeCParts(t *testing.T) {
	src := `package main

import (
	"unsafe"

	"<array>"
	"<stdio.h>"
	"<vector>"
)

type withC struct{ a, b, c C.int }

type withVec struct {
	n int
	v std.vector[int]
}

type withP struct {
	x int8
	f *stdio.FILE
}

type outer struct {
	x  int8
	in withC
}

type viaPtr struct {
	p *withC
	s []withC
	m map[int]withC
}

type mixedC struct {
	a int8
	b int64
	n C.int
	d int8
}

type embedsC struct {
	x int8
	mixedC
}

var ca [3]C.int
var mc mixedC
var ec embedsC

const (
	e1 = unsafe.Sizeof(withC{})
	e2 = unsafe.Sizeof(withP{})
	e3 = unsafe.Alignof(outer{})
	e4 = unsafe.Sizeof(ca)
	e5 = unsafe.Offsetof(mc.d)
	e6 = unsafe.Alignof(mc)
	e7 = unsafe.Offsetof(ec.b)
	e11 = unsafe.Sizeof(withVec{})
)

var e8 [unsafe.Sizeof(withC{})]byte
var e9 std.array[byte, unsafe.Sizeof(outer{})]

const (
	ok1 = unsafe.Sizeof(viaPtr{})
	ok2 = unsafe.Offsetof(mc.b)
	ok3 = unsafe.Sizeof(&ca)
)

var e10 std.array[byte, unsafe.Offsetof(mc.n)]
var ok4 std.array[byte, unsafe.Offsetof(mc.b)]

func main() { _, _, _, _ = e8, e9, e10, ok4 }
`
	root := writeTree(t, map[string]string{"gocpp.mod": "module example.com/m\n", "main.gp": src})
	prog, r := loadAndCheck(t, load.Config{}, root, ".")
	wantErrs := []struct{ marker, call, part string }{
		{"e1 =", "unsafe.Sizeof(withC{})", "field a C.int"},
		{"e2 =", "unsafe.Sizeof(withP{})", "field f *stdio.FILE"},
		{"e3 =", "unsafe.Alignof(outer{})", "field in.a C.int"},
		{"e4 =", "unsafe.Sizeof(ca)", "its element type"},
		{"e5 =", "unsafe.Offsetof(mc.d)", "field n C.int"},
		{"e6 =", "unsafe.Alignof(mc)", "field n C.int"},
		{"e7 =", "unsafe.Offsetof(ec.b)", "field mixedC.n C.int"},
		{"e11 =", "unsafe.Sizeof(withVec{})", "field v std.vector[int]"},
		{"var e8", "unsafe.Sizeof(withC{})", "field a C.int"},
		{"var e9", "unsafe.Sizeof(outer{})", "field in.a C.int"},
		{"var e10", "unsafe.Offsetof(mc.n)", "field n C.int"}, // n's alignment
	}
	want := "# example.com/m\n"
	const note = ": the size of a C/C++ type is known only to the C++ compiler, so it is a value, not a Go constant"
	for _, e := range wantErrs {
		line := lineOf(t, src, e.marker)
		col := strings.Index(strings.Split(src, "\n")[line-1], e.call) + 1
		what := "is not constant"
		switch {
		case strings.HasPrefix(e.marker, "var e8"):
			what = "array length " + e.call + " (value of type uintptr) must be constant"
			want += fmt.Sprintf("./main.gp:%d:%d: %s%s\n", line, col, what, note)
			continue
		case strings.HasPrefix(e.marker, "var e9"), strings.HasPrefix(e.marker, "var e10"):
			what = "template argument " + e.call + " (value of type uintptr) must be constant"
			want += fmt.Sprintf("./main.gp:%d:%d: %s%s\n", line, col, what, note)
			continue
		}
		want += fmt.Sprintf("./main.gp:%d:%d: %s (value of type uintptr) is not constant%s\n", line, col, e.call, note)
	}
	if errs := errorText(t, prog, r, root, true); errs != want {
		t.Fatalf("errors:\n%s\nwant:\n%s", errs, want)
	}
	p := mainPkg(t, prog, r)
	for name, want := range map[string]int64{"ok1": 72, "ok2": 8, "ok3": 16} {
		v := p.Types.Scope().Lookup(name).(*types.Const).Val()
		if got, ok := constant.Int64Val(v); !ok || got != want {
			t.Errorf("%s = %v, want %d", name, v, want)
		}
	}
	at := "unsafe.Offsetof(mc.b)]"
	e := findExpr(t, prog, p, lineOf(t, src, at), at, "unsafe.Offsetof(mc.b)")
	if got := p.Info.Types[e].Value; got == nil || !constant.Compare(got, token.EQL, constant.MakeInt64(8)) {
		t.Errorf("unsafe.Offsetof(mc.b) in an opaque argument is %v, want 8", got)
	}
}

// TestSizesNestedStructs: a chain of structs that each hold the next by value has a layout that is
// computed once for each level. The layout of a struct used to ask for the layout of each field
// three times, so the cost was 3^depth: a chain 12 deep took minutes to lower. With depth 40 the
// old code never finishes (the test would end with the timeout of go test); this one is instant.
func TestSizesNestedStructs(t *testing.T) {
	const depth = 40
	var src strings.Builder
	src.WriteString("package p\n\nimport \"unsafe\"\n\n")
	for i := range depth {
		if i+1 < depth {
			fmt.Fprintf(&src, "type S%d struct {\n\tv byte\n\tin S%d\n\tw int16\n}\n\n", i, i+1)
		} else {
			fmt.Fprintf(&src, "type S%d struct{ v int32 }\n\n", i)
		}
	}
	src.WriteString("const Size = unsafe.Sizeof(S0{})\nconst Align = unsafe.Alignof(S0{})\n")
	pkg := checkSizesSrc(t, "nested.go", src.String(), &sizesImporter{t: t}, nil)
	// S39 is 4 bytes (align 4); each level adds v at 0, `in` at its alignment, w after it and pads
	// to the alignment: the first level where `in` is 4-aligned gives 4+4... compute by hand below.
	size, align := int64(4), int64(4)
	for range depth - 1 {
		off := alignUp(1, align)
		off += size
		size = alignUp(alignUp(off, 2)+2, align)
	}
	if got := constInt(t, pkg, "Size"); got != size {
		t.Errorf("Sizeof(S0) = %d, want %d", got, size)
	}
	if got := constInt(t, pkg, "Align"); got != align {
		t.Errorf("Alignof(S0) = %d, want %d", got, align)
	}
}

func constInt(t *testing.T, pkg *types.Package, name string) int64 {
	t.Helper()
	c, ok := pkg.Scope().Lookup(name).(*types.Const)
	if !ok {
		t.Fatalf("no constant %s", name)
	}
	v, ok := constant.Int64Val(c.Val())
	if !ok {
		t.Fatalf("constant %s is not an int64: %v", name, c.Val())
	}
	return v
}
