package check

import "go/types"

// gocppSizes gives go/types the sizes and alignments of gocpp's types, for
// unsafe.Sizeof, unsafe.Alignof and unsafe.Offsetof, and for constant
// expressions that use them (types.Config.Sizes).
//
// It is a stub for now: it returns what Go's gc compiler uses on amd64
// (types.SizesFor("gc", "amd64")), on every target. That is right for the
// numeric types and bool (int is 8 bytes, as gocpp's int is int64_t), and
// for structs and arrays of them, but not for the types that become
// runtime types with another layout (Phase 03 README §2.1): a string is a
// 32-byte gp::string, a pointer a 16-byte gp::ptr, a slice 40 bytes, a map
// 16. Phase 03 replaces the stub with the real layout of the generated
// C++, and its static_asserts keep both sides in sync.
//
// One place doesn't use it: types.CheckExpr, which checks the arguments of
// opaque instantiations (checkOpaqueArgs), takes no types.Config and uses
// go/types' default sizes, gc on amd64. That is the stub's layout, so the
// two agree now; when the real layout comes, unsafe.Sizeof(s) in
// std.array[byte, unsafe.Sizeof(s)] would silently be Go's size.
// TestOpaqueArgSizes fails then, on purpose.
type gocppSizes struct{}

// gcAMD64 is the layout that gocppSizes returns for now.
var gcAMD64 = types.SizesFor("gc", "amd64")

func (gocppSizes) Alignof(t types.Type) int64 { return gcAMD64.Alignof(t) }

func (gocppSizes) Offsetsof(fields []*types.Var) []int64 { return gcAMD64.Offsetsof(fields) }

func (gocppSizes) Sizeof(t types.Type) int64 { return gcAMD64.Sizeof(t) }
