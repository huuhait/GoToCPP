// gocpp/sizes.hpp: the sizes and alignments of gocpp's types, checked.
//
// GENERATED from the size table in internal/check/sizes.go (runtimeTypes): don't edit it, change
// the table and run go test ./internal/check -run TestSizesHeader -update-sizes.
//
// go/types gets these layouts from gocppSizes, so unsafe.Sizeof, Alignof and Offsetof give what
// the C++ gives (divergence D15 in docs/spec/language.md). These static_asserts keep the C++ in
// step with the table. Structs and arrays are computed from them (runtime/test/layout.cpp).
#pragma once

#include <gocpp/string.hpp>
#include <gocpp/ptr.hpp>

#include <cstdint>

static_assert(sizeof(bool) == 1, "bool: 1 byte");
static_assert(alignof(bool) == 1, "bool: alignment 1");
static_assert(sizeof(int8_t) == 1, "int8: 1 byte");
static_assert(alignof(int8_t) == 1, "int8: alignment 1");
static_assert(sizeof(int16_t) == 2, "int16: 2 bytes");
static_assert(alignof(int16_t) == 2, "int16: alignment 2");
static_assert(sizeof(int32_t) == 4, "int32: 4 bytes");
static_assert(alignof(int32_t) == 4, "int32: alignment 4");
static_assert(sizeof(int64_t) == 8, "int64, int: 8 bytes");
static_assert(alignof(int64_t) == 8, "int64, int: alignment 8");
static_assert(sizeof(uint8_t) == 1, "uint8: 1 byte");
static_assert(alignof(uint8_t) == 1, "uint8: alignment 1");
static_assert(sizeof(uint16_t) == 2, "uint16: 2 bytes");
static_assert(alignof(uint16_t) == 2, "uint16: alignment 2");
static_assert(sizeof(uint32_t) == 4, "uint32: 4 bytes");
static_assert(alignof(uint32_t) == 4, "uint32: alignment 4");
static_assert(sizeof(uint64_t) == 8, "uint64, uint: 8 bytes");
static_assert(alignof(uint64_t) == 8, "uint64, uint: alignment 8");
static_assert(sizeof(uintptr_t) == 8, "uintptr: 8 bytes");
static_assert(alignof(uintptr_t) == 8, "uintptr: alignment 8");
static_assert(sizeof(float) == 4, "float32: 4 bytes");
static_assert(alignof(float) == 4, "float32: alignment 4");
static_assert(sizeof(double) == 8, "float64: 8 bytes");
static_assert(alignof(double) == 8, "float64: alignment 8");
static_assert(sizeof(gp::string) == 32, "string: 32 bytes");
static_assert(alignof(gp::string) == 8, "string: alignment 8");
static_assert(sizeof(gp::ptr<int64_t>) == 16, "*int: 16 bytes");
static_assert(alignof(gp::ptr<int64_t>) == 8, "*int: alignment 8");
static_assert(sizeof(gp::ptr<void>) == 16, "unsafe.Pointer: 16 bytes");
static_assert(alignof(gp::ptr<void>) == 8, "unsafe.Pointer: alignment 8");
static_assert(sizeof(gp::weak_ptr<int64_t>) == 24, "weak.Pointer[int]: 24 bytes");
static_assert(alignof(gp::weak_ptr<int64_t>) == 8, "weak.Pointer[int]: alignment 8");

// Not in the runtime yet. When a step adds one of these types, TestSizesTable fails until
// its row in the table is no longer pending; then this file asserts it.
//   complex64, std::complex<float>: 8 bytes, alignment 4 (complex numbers)
//   complex128, std::complex<double>: 16 bytes, alignment 8 (complex numbers)
//   []int, gp::slice<int64_t>: 40 bytes, alignment 8 (step 3.5)
//   [0]int64, gp::array<int64_t, 0>: 1 byte, alignment 1 (step 3.6)
//   [3]int16, gp::array<int16_t, 3>: 6 bytes, alignment 2 (step 3.6)
//   map[int]int, gp::map<int64_t, int64_t>: 16 bytes, alignment 8 (step 3.7)
//   func(), gp::func<void()>: 32 bytes, alignment 8 (step 3.11)
//   any, gp::any: 32 bytes, alignment 8 (Phase 05)
//   chan int, gp::chan<int64_t>: 16 bytes, alignment 8 (Phase 08)
