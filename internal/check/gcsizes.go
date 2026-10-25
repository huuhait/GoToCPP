package check

// The GC engine's size table (phase-16 README §2.2 and §2.8; job 16.1e), separate from the
// reference-counting engine's table in sizes.go: the GC engine's values have Go's sizes
// (string 16, slice 24, interface 16, func/map/chan 8), not gocpp's (32, 40, 32, 24, 16, 16).
// Nothing here changes gocppSizes or the ARC engine's sizes.
//
// gcRuntimeTypes is the table that gcSizesHPP (gcsizes_test.go) turns into
// runtime/include/gocpp/gc/sizes.hpp: a static_assert of each C++ type's size, alignment and
// pointer-ness, against Go's numbers. gcLayoutSrc is a Go source with the struct shapes whose
// C++ mirrors gc_layout_test.cpp checks, including the zero-size shapes of sync, sync/atomic,
// weak and log/slog; TestGCLayoutShapes (gcsizes_test.go) checks every number against real Go.
//
// The numbers come from Go 1.25's gc on amd64 (types.SizesFor("gc", "amd64"), the same sizes
// unsafe.Sizeof reports); internal/check/testdata/gclayout/main.go is the small program that
// prints them for the standard types.

// gcRuntimeType is a row of the GC engine's size table: a Go type, the C++ type that stands
// for it, and Go's layout.
type gcRuntimeType struct {
	goType string // the Go type, for the comment and the test
	cpp    string // the C++ type that sizes.hpp asserts
	layout layout // Go's size and alignment
	ptrs   bool   // Go's type holds a pointer (go_layout<T>().ptrs)
}

// gcRuntimeTypes is the GC engine's size table, the one source of the layouts that
// runtime/include/gocpp/gc/sizes.hpp asserts.
var gcRuntimeTypes = []gcRuntimeType{
	{goType: "bool", cpp: "bool", layout: layout{1, 1}},
	{goType: "int8", cpp: "int8_t", layout: layout{1, 1}},
	{goType: "int16", cpp: "int16_t", layout: layout{2, 2}},
	{goType: "int32", cpp: "int32_t", layout: layout{4, 4}},
	{goType: "int64", cpp: "int64_t", layout: layout{8, 8}},
	{goType: "int", cpp: "int64_t", layout: layout{8, 8}},
	{goType: "uint8", cpp: "uint8_t", layout: layout{1, 1}},
	{goType: "uint16", cpp: "uint16_t", layout: layout{2, 2}},
	{goType: "uint32", cpp: "uint32_t", layout: layout{4, 4}},
	{goType: "uint64", cpp: "uint64_t", layout: layout{8, 8}},
	{goType: "uint", cpp: "uint64_t", layout: layout{8, 8}},
	{goType: "uintptr", cpp: "uintptr_t", layout: layout{8, 8}},
	{goType: "float32", cpp: "float", layout: layout{4, 4}},
	{goType: "float64", cpp: "double", layout: layout{8, 8}},
	{goType: "string", cpp: "gp::gc::string", layout: layout{16, 8}, ptrs: true},
	{goType: "*int", cpp: "int64_t*", layout: layout{8, 8}, ptrs: true},
	{goType: "unsafe.Pointer", cpp: "void*", layout: layout{8, 8}, ptrs: true},
	{goType: "[]int", cpp: "gp::gc::slice<int64_t>", layout: layout{24, 8}, ptrs: true},
	{goType: "[0]int64", cpp: "gp::gc::array<int64_t, 0>", layout: layout{0, 8}},
	{goType: "[3]int16", cpp: "gp::gc::array<int16_t, 3>", layout: layout{6, 2}},
	{goType: "map[int]int", cpp: "gp::gc::map<int64_t, int64_t>", layout: layout{8, 8},
		ptrs: true},
	{goType: "func()", cpp: "gp::gc::func<void()>", layout: layout{8, 8}, ptrs: true},
	{goType: "any", cpp: "gp::gc::eface", layout: layout{16, 8}, ptrs: true},
	{goType: "interface{ M() }", cpp: "gp::gc::iface", layout: layout{16, 8}, ptrs: true},
	{goType: "chan int", cpp: "gp::gc::chan<int64_t>", layout: layout{8, 8}, ptrs: true},
	{goType: "struct{}", cpp: "gp::gc::zero_field<0>", layout: layout{0, 1}},
}

// gcLayoutSrc holds the structs that runtime/test/gc_layout_test.cpp checks. It is a Go source,
// so go/types (with Go's own sizes) computes the layouts; the generated C++ mirrors each one
// with the runtime's helpers, and its static_asserts prove that the C++ has the same size,
// alignment and field offsets. The shapes:
//   - the zero-size cases: an empty struct, a zero-size field first, last and in the middle, two
//     zero-size fields, and [0]T with an alignment;
//   - the zero-size shapes of the standard library: sync.Mutex (noCopy), sync.WaitGroup,
//     sync.RWMutex, sync.Once, sync.Pool, sync/atomic.Pointer ([0]*T), sync/atomic.Int64
//     (align64), weak.Pointer ([0]*T) and log/slog.Value ([0]func()) and Attr.
//
// TestGCLayoutShapes checks each mirror against the real standard type with reflect and unsafe;
// internal/check/testdata/gclayout/main.go prints the real numbers.
const gcLayoutSrc = `package gclayout

type empty struct{}

// The zero-size shapes.

type zLast struct {
	A int64
	Z empty
}

type zFirst struct {
	Z empty
	A int64
}

type zz struct {
	Z1 empty
	Z2 empty
}

type arrZLast struct {
	A int32
	Z [0]int64
}

type arrZFirst struct {
	Z [0]int64
	A int32
}

type arrZMid struct {
	A int32
	Z [0]int64
	B int32
}

type zeroSizeNamed struct {
	z [0]int64
}

type nestedZero struct {
	A int32
	Z zeroSizeNamed
	B int32
}

// The standard library's zero-size shapes, with the fields of Go 1.25.

type syncMutexInner struct {
	state int32
	sema  uint32
}

type syncMutex struct {
	noCopy empty
	mu     syncMutexInner
}

type syncWaitGroup struct {
	noCopy empty
	state  uint64
	sema   uint32
}

type syncRWMutex struct {
	w           syncMutexInner
	writerSem   uint32
	readerSem   uint32
	readerCount int32
	readerWait  int32
}

type syncOnce struct {
	noCopy empty
	done   uint32
	m      syncMutexInner
}

type syncPool struct {
	noCopy    empty
	local     *int64
	localSize uint64
	victim    *int64
	victimSize uint64
	New       func() any
}

type atomicPointer struct {
	z      [0]*int64
	noCopy empty
	v      *int64
}

type atomicInt64 struct {
	noCopy  empty
	align64 [0]uint64
	v       int64
}

type weakPointer struct {
	z [0]*int64
	u *int64
}

type slogValue struct {
	noCompare [0]func()
	num       uint64
	any       any
}

type slogAttr struct {
	Key   string
	Value slogValue
}
`
