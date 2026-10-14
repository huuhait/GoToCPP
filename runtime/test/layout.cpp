// Struct layouts, checked: the C++ structs that the emitter writes for Go structs (the fields
// in source order), with the sizes, alignments and offsets that go/types gets from gocppSizes.
//
// GENERATED from internal/check/sizes_test.go (layoutSrc): don't edit it, run go test
// ./internal/check -run TestSizesHeader -update-sizes. runtime/layout_test.go compiles it with
// g++ and clang++, and TestWindowsSyntax with MinGW-w64; it has no code to run.
#include <gocpp/rt.hpp>

#include <complex>
#include <cstddef>
#include <cstdint>

static_assert(sizeof(std::complex<float>) == 8 && alignof(std::complex<float>) == 4);
static_assert(sizeof(std::complex<double>) == 16 && alignof(std::complex<double>) == 8);

namespace layout_test {

struct Empty {};
static_assert(sizeof(Empty) == 1 && alignof(Empty) == 1);

struct Small {
    int8_t a;
    int16_t b;
    int8_t c;
};
static_assert(sizeof(Small) == 6 && alignof(Small) == 2);
static_assert(offsetof(Small, a) == 0);
static_assert(offsetof(Small, b) == 2);
static_assert(offsetof(Small, c) == 4);

struct Mixed {
    int8_t a = 0;
    gp::string s;
    gp::ptr<int64_t> p;
    bool b = false;
    bool operator==(const Mixed&) const = default;
};
static_assert(sizeof(Mixed) == 64 && alignof(Mixed) == 8);
static_assert(offsetof(Mixed, a) == 0);
static_assert(offsetof(Mixed, s) == 8);
static_assert(offsetof(Mixed, p) == 40);
static_assert(offsetof(Mixed, b) == 56);

struct Tail {
    gp::string s;
    int8_t b;
};
static_assert(sizeof(Tail) == 40 && alignof(Tail) == 8);
static_assert(offsetof(Tail, s) == 0);
static_assert(offsetof(Tail, b) == 32);

struct Nested {
    int8_t x;
    Mixed in;
    Empty e;
    Tail t;
    float f;
    Small sm;
};
static_assert(sizeof(Nested) == 136 && alignof(Nested) == 8);
static_assert(offsetof(Nested, x) == 0);
static_assert(offsetof(Nested, in) == 8);
static_assert(offsetof(Nested, e) == 72);
static_assert(offsetof(Nested, t) == 80);
static_assert(offsetof(Nested, f) == 120);
static_assert(offsetof(Nested, sm) == 124);

struct EmptyFirst {
    Empty e;
    int64_t n;
};
static_assert(sizeof(EmptyFirst) == 16 && alignof(EmptyFirst) == 8);
static_assert(offsetof(EmptyFirst, e) == 0);
static_assert(offsetof(EmptyFirst, n) == 8);

struct EmptyLast {
    int64_t n;
    Empty e;
};
static_assert(sizeof(EmptyLast) == 16 && alignof(EmptyLast) == 8);
static_assert(offsetof(EmptyLast, n) == 0);
static_assert(offsetof(EmptyLast, e) == 8);

struct Pointers {
    bool b;
    gp::weak_ptr<Mixed> w;
    gp::ptr<void> u;
    gp::ptr<Nested> pn;
    int32_t r;
    gp::ptr<gp::ptr<Small>> pp;
};
static_assert(sizeof(Pointers) == 88 && alignof(Pointers) == 8);
static_assert(offsetof(Pointers, b) == 0);
static_assert(offsetof(Pointers, w) == 8);
static_assert(offsetof(Pointers, u) == 32);
static_assert(offsetof(Pointers, pn) == 48);
static_assert(offsetof(Pointers, r) == 64);
static_assert(offsetof(Pointers, pp) == 72);

} // namespace layout_test
