// gocpp/gc/sizes.hpp: the sizes, alignments and pointer-ness of the GC engine's types,
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

static_assert(sizeof(bool) == 1, "bool: 1 byte");
static_assert(alignof(bool) == 1, "bool: alignment 1");
static_assert(gp::gc::go_layout<bool>().size == 1, "bool: Go size");
static_assert(gp::gc::go_layout<bool>().align == 1, "bool: Go alignment");
static_assert(gp::gc::go_layout<bool>().ptrs == false, "bool: Go pointer-ness");
static_assert(sizeof(int8_t) == 1, "int8: 1 byte");
static_assert(alignof(int8_t) == 1, "int8: alignment 1");
static_assert(gp::gc::go_layout<int8_t>().size == 1, "int8: Go size");
static_assert(gp::gc::go_layout<int8_t>().align == 1, "int8: Go alignment");
static_assert(gp::gc::go_layout<int8_t>().ptrs == false, "int8: Go pointer-ness");
static_assert(sizeof(int16_t) == 2, "int16: 2 bytes");
static_assert(alignof(int16_t) == 2, "int16: alignment 2");
static_assert(gp::gc::go_layout<int16_t>().size == 2, "int16: Go size");
static_assert(gp::gc::go_layout<int16_t>().align == 2, "int16: Go alignment");
static_assert(gp::gc::go_layout<int16_t>().ptrs == false, "int16: Go pointer-ness");
static_assert(sizeof(int32_t) == 4, "int32: 4 bytes");
static_assert(alignof(int32_t) == 4, "int32: alignment 4");
static_assert(gp::gc::go_layout<int32_t>().size == 4, "int32: Go size");
static_assert(gp::gc::go_layout<int32_t>().align == 4, "int32: Go alignment");
static_assert(gp::gc::go_layout<int32_t>().ptrs == false, "int32: Go pointer-ness");
static_assert(sizeof(int64_t) == 8, "int64, int: 8 bytes");
static_assert(alignof(int64_t) == 8, "int64, int: alignment 8");
static_assert(gp::gc::go_layout<int64_t>().size == 8, "int64, int: Go size");
static_assert(gp::gc::go_layout<int64_t>().align == 8, "int64, int: Go alignment");
static_assert(gp::gc::go_layout<int64_t>().ptrs == false, "int64, int: Go pointer-ness");
static_assert(sizeof(uint8_t) == 1, "uint8: 1 byte");
static_assert(alignof(uint8_t) == 1, "uint8: alignment 1");
static_assert(gp::gc::go_layout<uint8_t>().size == 1, "uint8: Go size");
static_assert(gp::gc::go_layout<uint8_t>().align == 1, "uint8: Go alignment");
static_assert(gp::gc::go_layout<uint8_t>().ptrs == false, "uint8: Go pointer-ness");
static_assert(sizeof(uint16_t) == 2, "uint16: 2 bytes");
static_assert(alignof(uint16_t) == 2, "uint16: alignment 2");
static_assert(gp::gc::go_layout<uint16_t>().size == 2, "uint16: Go size");
static_assert(gp::gc::go_layout<uint16_t>().align == 2, "uint16: Go alignment");
static_assert(gp::gc::go_layout<uint16_t>().ptrs == false, "uint16: Go pointer-ness");
static_assert(sizeof(uint32_t) == 4, "uint32: 4 bytes");
static_assert(alignof(uint32_t) == 4, "uint32: alignment 4");
static_assert(gp::gc::go_layout<uint32_t>().size == 4, "uint32: Go size");
static_assert(gp::gc::go_layout<uint32_t>().align == 4, "uint32: Go alignment");
static_assert(gp::gc::go_layout<uint32_t>().ptrs == false, "uint32: Go pointer-ness");
static_assert(sizeof(uint64_t) == 8, "uint64, uint: 8 bytes");
static_assert(alignof(uint64_t) == 8, "uint64, uint: alignment 8");
static_assert(gp::gc::go_layout<uint64_t>().size == 8, "uint64, uint: Go size");
static_assert(gp::gc::go_layout<uint64_t>().align == 8, "uint64, uint: Go alignment");
static_assert(gp::gc::go_layout<uint64_t>().ptrs == false, "uint64, uint: Go pointer-ness");
static_assert(sizeof(uintptr_t) == 8, "uintptr: 8 bytes");
static_assert(alignof(uintptr_t) == 8, "uintptr: alignment 8");
static_assert(gp::gc::go_layout<uintptr_t>().size == 8, "uintptr: Go size");
static_assert(gp::gc::go_layout<uintptr_t>().align == 8, "uintptr: Go alignment");
static_assert(gp::gc::go_layout<uintptr_t>().ptrs == false, "uintptr: Go pointer-ness");
static_assert(sizeof(float) == 4, "float32: 4 bytes");
static_assert(alignof(float) == 4, "float32: alignment 4");
static_assert(gp::gc::go_layout<float>().size == 4, "float32: Go size");
static_assert(gp::gc::go_layout<float>().align == 4, "float32: Go alignment");
static_assert(gp::gc::go_layout<float>().ptrs == false, "float32: Go pointer-ness");
static_assert(sizeof(double) == 8, "float64: 8 bytes");
static_assert(alignof(double) == 8, "float64: alignment 8");
static_assert(gp::gc::go_layout<double>().size == 8, "float64: Go size");
static_assert(gp::gc::go_layout<double>().align == 8, "float64: Go alignment");
static_assert(gp::gc::go_layout<double>().ptrs == false, "float64: Go pointer-ness");
static_assert(sizeof(gp::gc::string) == 16, "string: 16 bytes");
static_assert(alignof(gp::gc::string) == 8, "string: alignment 8");
static_assert(gp::gc::go_layout<gp::gc::string>().size == 16, "string: Go size");
static_assert(gp::gc::go_layout<gp::gc::string>().align == 8, "string: Go alignment");
static_assert(gp::gc::go_layout<gp::gc::string>().ptrs == true, "string: Go pointer-ness");
static_assert(sizeof(int64_t*) == 8, "*int: 8 bytes");
static_assert(alignof(int64_t*) == 8, "*int: alignment 8");
static_assert(gp::gc::go_layout<int64_t*>().size == 8, "*int: Go size");
static_assert(gp::gc::go_layout<int64_t*>().align == 8, "*int: Go alignment");
static_assert(gp::gc::go_layout<int64_t*>().ptrs == true, "*int: Go pointer-ness");
static_assert(sizeof(void*) == 8, "unsafe.Pointer: 8 bytes");
static_assert(alignof(void*) == 8, "unsafe.Pointer: alignment 8");
static_assert(gp::gc::go_layout<void*>().size == 8, "unsafe.Pointer: Go size");
static_assert(gp::gc::go_layout<void*>().align == 8, "unsafe.Pointer: Go alignment");
static_assert(gp::gc::go_layout<void*>().ptrs == true, "unsafe.Pointer: Go pointer-ness");
static_assert(sizeof(gp::gc::slice<int64_t>) == 24, "[]int: 24 bytes");
static_assert(alignof(gp::gc::slice<int64_t>) == 8, "[]int: alignment 8");
static_assert(gp::gc::go_layout<gp::gc::slice<int64_t>>().size == 24, "[]int: Go size");
static_assert(gp::gc::go_layout<gp::gc::slice<int64_t>>().align == 8, "[]int: Go alignment");
static_assert(gp::gc::go_layout<gp::gc::slice<int64_t>>().ptrs == true, "[]int: Go pointer-ness");
static_assert(gp::gc::go_layout<gp::gc::array<int64_t, 0>>().size == 0, "[0]int64: Go size 0");
static_assert(alignof(gp::gc::array<int64_t, 0>) == 8, "[0]int64: alignment 8");
static_assert(sizeof(gp::gc::array<int64_t, 0>) >= 1, "[0]int64: a C++ object is at least 1 byte");
static_assert(sizeof(gp::gc::array<int16_t, 3>) == 6, "[3]int16: 6 bytes");
static_assert(alignof(gp::gc::array<int16_t, 3>) == 2, "[3]int16: alignment 2");
static_assert(gp::gc::go_layout<gp::gc::array<int16_t, 3>>().size == 6, "[3]int16: Go size");
static_assert(gp::gc::go_layout<gp::gc::array<int16_t, 3>>().align == 2, "[3]int16: Go alignment");
static_assert(gp::gc::go_layout<gp::gc::array<int16_t, 3>>().ptrs == false,
              "[3]int16: Go pointer-ness");
static_assert(sizeof(gp::gc::map<int64_t, int64_t>) == 8, "map[int]int: 8 bytes");
static_assert(alignof(gp::gc::map<int64_t, int64_t>) == 8, "map[int]int: alignment 8");
static_assert(gp::gc::go_layout<gp::gc::map<int64_t, int64_t>>().size == 8, "map[int]int: Go size");
static_assert(gp::gc::go_layout<gp::gc::map<int64_t, int64_t>>().align == 8,
              "map[int]int: Go alignment");
static_assert(gp::gc::go_layout<gp::gc::map<int64_t, int64_t>>().ptrs == true,
              "map[int]int: Go pointer-ness");
static_assert(sizeof(gp::gc::func<void()>) == 8, "func(): 8 bytes");
static_assert(alignof(gp::gc::func<void()>) == 8, "func(): alignment 8");
static_assert(gp::gc::go_layout<gp::gc::func<void()>>().size == 8, "func(): Go size");
static_assert(gp::gc::go_layout<gp::gc::func<void()>>().align == 8, "func(): Go alignment");
static_assert(gp::gc::go_layout<gp::gc::func<void()>>().ptrs == true, "func(): Go pointer-ness");
static_assert(sizeof(gp::gc::eface) == 16, "any: 16 bytes");
static_assert(alignof(gp::gc::eface) == 8, "any: alignment 8");
static_assert(gp::gc::go_layout<gp::gc::eface>().size == 16, "any: Go size");
static_assert(gp::gc::go_layout<gp::gc::eface>().align == 8, "any: Go alignment");
static_assert(gp::gc::go_layout<gp::gc::eface>().ptrs == true, "any: Go pointer-ness");
static_assert(sizeof(gp::gc::iface) == 16, "interface{ M() }: 16 bytes");
static_assert(alignof(gp::gc::iface) == 8, "interface{ M() }: alignment 8");
static_assert(gp::gc::go_layout<gp::gc::iface>().size == 16, "interface{ M() }: Go size");
static_assert(gp::gc::go_layout<gp::gc::iface>().align == 8, "interface{ M() }: Go alignment");
static_assert(gp::gc::go_layout<gp::gc::iface>().ptrs == true, "interface{ M() }: Go pointer-ness");
static_assert(sizeof(gp::gc::chan<int64_t>) == 8, "chan int: 8 bytes");
static_assert(alignof(gp::gc::chan<int64_t>) == 8, "chan int: alignment 8");
static_assert(gp::gc::go_layout<gp::gc::chan<int64_t>>().size == 8, "chan int: Go size");
static_assert(gp::gc::go_layout<gp::gc::chan<int64_t>>().align == 8, "chan int: Go alignment");
static_assert(gp::gc::go_layout<gp::gc::chan<int64_t>>().ptrs == true, "chan int: Go pointer-ness");
static_assert(gp::gc::go_layout<gp::gc::zero_field<0>>().size == 0, "struct{}: Go size 0");
static_assert(alignof(gp::gc::zero_field<0>) == 1, "struct{}: alignment 1");
static_assert(sizeof(gp::gc::zero_field<0>) >= 1, "struct{}: a C++ object is at least 1 byte");
