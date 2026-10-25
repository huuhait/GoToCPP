// The unit tests of the GC engine's values (phase-16 README §2.2 and §2.8; job 16.1e): Go's
// slices, arrays, interfaces, funcs, maps and channels in Go's exact layout, the struct-layout
// helpers, new_, make, append with Go's growth, and Go's nil, bounds, division and shift panics.
//
// runtime/cxx_test.go compiles this file with the GC engine's sources (rtfiles.GCSources) with
// g++ and clang++, at -O0, at -O2 and with ASan + UBSan, and runs it like test/unit.cpp: it
// prints "ok: N checks" and exits 0, or prints each failed check and exits 1.
//
// The numbers that append gives are real Go's runtime.growslice (not the compiler's inline
// append, which may allocate more): internal/check/testdata/gclayout/main.go prints them (with
// reflect, which calls growslice), and runtime/test/gc_layout_test.cpp checks the struct
// layouts.
#include <gocpp/gc/rt.hpp>
#include <gocpp/gc/sizes.hpp>

#include <cstdint>
#include <cstdio>
#include <cstring>
#include <string>
#include <utility>
#include <vector>

namespace {

int checks = 0;
int failures = 0;

void report(bool ok, const char* file, int line, const std::string& what) {
    checks++;
    if (!ok) {
        failures++;
        std::fprintf(stderr, "%s:%d: %s\n", file, line, what.c_str());
    }
}

#define CHECK(...) report((__VA_ARGS__), __FILE__, __LINE__, "CHECK(" #__VA_ARGS__ ") failed")

// check_panic runs f, which must throw a gp::gc::panic_error with Go's message and dynamic type.
template <typename F>
void check_panic(const char* what, const char* want, const char* type, F f) {
    checks++;
    try {
        f();
    } catch (const gp::gc::panic_error& p) {
        if (p.gp_message != want) {
            failures++;
            std::fprintf(stderr, "%s: panic %s, want %s\n", what, p.gp_message.c_str(), want);
        } else if (p.gp_type != type) {
            failures++;
            std::fprintf(stderr, "%s: panic type %s, want %s\n", what, p.gp_type.c_str(), type);
        }
        return;
    } catch (...) {
        failures++;
        std::fprintf(stderr, "%s: a non-panic exception\n", what);
        return;
    }
    failures++;
    std::fprintf(stderr, "%s: no panic\n", what);
}

// The malloc backend never frees, so the test keeps every allocation reachable: LeakSanitizer
// (the asan mode) must not report the test's own intentional leaks. The list is never destroyed
// (the same pattern as gc_string_test.cpp). The bdwgc backend of job 16.1d collects, and then
// this keeps nothing.
std::vector<const void*>& keep() {
    static auto* list = new std::vector<const void*>;
    return *list;
}

template <typename T>
gp::gc::slice<T> keep_slice(gp::gc::slice<T> s) {
    if (s.gp_data() != nullptr) {
        keep().push_back(s.gp_data());
    }
    return s;
}

template <typename T>
T* keep_ptr(T* p) {
    keep().push_back(p);
    return p;
}

gp::gc::eface keep_eface(gp::gc::eface e) {
    if (e.gp_data() != nullptr) {
        keep().push_back(e.gp_data());
    }
    return e;
}

gp::gc::iface keep_iface(gp::gc::iface i) {
    if (i.gp_data() != nullptr) {
        keep().push_back(i.gp_data());
    }
    return i;
}

gp::gc::string keep_string(gp::gc::string s) {
    if (s.gp_data() != nullptr) {
        keep().push_back(s.gp_data());
    }
    return s;
}

// A struct as the emitter writes it: fields in Go order, gp_fields, and the layout checks.
struct point {
    std::int64_t X;
    std::int64_t Y;
    static constexpr auto gp_fields =
        gp::gc::fields(gp::gc::field(&point::X, "X"), gp::gc::field(&point::Y, "Y"));
};
GP_GC_LAYOUT(point, 16, 8);
GP_GC_FIELD_AT(point, X, 0);
GP_GC_FIELD_AT(point, Y, 8);

// A struct with a trailing zero-size field: Go's size is 16 (8 + the +1 byte, rounded up).
struct with_zero {
    std::int64_t A;
    [[no_unique_address]] gp::gc::zero_field<0> Z;
    gp::gc::padding<1> gp_tail;
    static constexpr auto gp_fields =
        gp::gc::fields(gp::gc::field(&with_zero::A, "A"), gp::gc::field(&with_zero::Z, "Z"));
};
GP_GC_LAYOUT(with_zero, 16, 8);
GP_GC_FIELD_AT(with_zero, A, 0);

// A closure object: the call function first, then the captured variables (job 16.1f writes
// these).
using add_sig = std::int64_t(std::int64_t, std::int64_t);

struct add_closure {
    gp::gc::closure<add_sig> gp_base;
    std::int64_t addend;
};

std::int64_t add_call(void* self, std::int64_t a, std::int64_t b) {
    auto* c = static_cast<add_closure*>(self);
    return a + b + c->addend;
}

// check_caps checks the capacity of a grown slice at selected lengths.
template <typename T>
void check_caps(const char* what, gp::gc::slice<T> s, int64_t up_to,
                std::initializer_list<std::pair<int64_t, int64_t>> want) {
    for (std::int64_t i = 1; i <= up_to; i++) {
        s = keep_slice(gp::gc::append(s, T{}));
        for (const auto& [len, cap] : want) {
            if (i == len) {
                checks++;
                if (s.gp_cap() != cap) {
                    failures++;
                    std::fprintf(stderr, "%s at len %lld: cap %lld, want %lld\n", what,
                                 static_cast<long long>(i), static_cast<long long>(s.gp_cap()),
                                 static_cast<long long>(cap));
                }
            }
        }
    }
}

} // namespace

int main() {
    // The runtime's panics throw to the test instead of printing Go's line and exiting (the
    // default until step 16.6; gocpp/gc/panic.hpp).
    gp::gc::detail::panic_hook = [](const gp::gc::panic_error& p) { throw p; };
    using gp::gc::array;
    using gp::gc::eface;
    using gp::gc::func;
    using gp::gc::iface;
    using gp::gc::map;
    using gp::gc::slice;
    using gp::gc::string;
    using gp::gc::to_eface;
    using gp::gc::to_iface;

    // Go's sizes (README §2.2). gc/sizes.hpp asserts them at compile time; these are the
    // runtime checks.
    CHECK(sizeof(string) == 16);
    CHECK(sizeof(slice<std::int64_t>) == 24);
    CHECK(sizeof(array<std::int64_t, 3>) == 24);
    CHECK(sizeof(eface) == 16);
    CHECK(sizeof(iface) == 16);
    CHECK(sizeof(func<void()>) == 8);
    CHECK(sizeof(map<std::int64_t, std::int64_t>) == 8);
    CHECK(sizeof(gp::gc::chan<std::int64_t>) == 8);
    CHECK(gp::gc::go_layout<point>().size == 16 && gp::gc::go_layout<point>().align == 8);
    CHECK(gp::gc::go_layout<array<std::int64_t, 0>>().size == 0);
    CHECK(gp::gc::go_layout<gp::gc::zero_field<0>>().size == 0);

    // --- slices ---

    // nil and empty.
    slice<std::int64_t> nil_slice;
    CHECK(nil_slice == nullptr);
    CHECK(gp::gc::len(nil_slice) == 0 && gp::gc::cap(nil_slice) == 0);
    slice<std::int64_t> empty = gp::gc::make_slice<std::int64_t>(0);
    CHECK(empty != nullptr);
    CHECK(gp::gc::len(empty) == 0 && gp::gc::cap(empty) == 0);
    slice<std::int64_t> empty_lit = gp::gc::slice_of<std::int64_t>({});
    CHECK(empty_lit != nullptr);

    // make and indexing.
    slice<std::int64_t> s = keep_slice(gp::gc::make_slice<std::int64_t>(3, 5));
    CHECK(gp::gc::len(s) == 3 && gp::gc::cap(s) == 5);
    for (std::int64_t i = 0; i < 3; i++) {
        CHECK(s[i] == 0); // make gives zero values
    }
    s[0] = 10;
    s[1] = 20;
    s[2] = 30;
    CHECK(s[0] == 10 && s[2] == 30);
    CHECK(gp::gc::len(s) == 3 && gp::gc::cap(s) == 5);

    // Bounds checks, with Go's messages and panic type.
    check_panic("index 5 of 3", "runtime error: index out of range [5] with length 3",
                "runtime.boundsError", [&] { (void)s[5]; });
    check_panic("index -1", "runtime error: index out of range [-1]", "runtime.boundsError",
                [&] { (void)s[-1]; });
    check_panic("index unsigned 5", "runtime error: index out of range [5] with length 3",
                "runtime.boundsError", [&] { (void)s[std::uint64_t(5)]; });
    check_panic("slice hi > cap", "runtime error: slice bounds out of range [:6] with capacity 5",
                "runtime.boundsError", [&] { (void)gp::gc::sub(s, 0, 6); });
    check_panic("slice lo > hi", "runtime error: slice bounds out of range [2:1]",
                "runtime.boundsError", [&] { (void)gp::gc::sub(s, 2, 1); });
    check_panic("slice3 hi > max", "runtime error: slice bounds out of range [:4:2]",
                "runtime.boundsError", [&] { (void)gp::gc::sub(s, 0, 4, 2); });

    // Slicing, and Go's no-past-the-end rule: a result whose capacity is zero keeps the base
    // pointer (golang.org/issue/8404).
    slice<std::int64_t> base = s;
    slice<std::int64_t> t = gp::gc::sub(s, 1, 3);
    CHECK(gp::gc::len(t) == 2 && gp::gc::cap(t) == 4);
    CHECK(t.gp_data() == base.gp_data() + 1);
    CHECK(t[0] == 20 && t[1] == 30);
    slice<std::int64_t> end = gp::gc::sub(s, 5, 5);
    CHECK(gp::gc::len(end) == 0 && gp::gc::cap(end) == 0);
    CHECK(end.gp_data() == base.gp_data()); // not base + 5 (past the array)
    slice<std::int64_t> z3 = gp::gc::sub(s, 2, 2, 2);
    CHECK(z3.gp_cap() == 0 && z3.gp_data() == base.gp_data());
    slice<std::int64_t> one = gp::gc::sub(s, 2, 3, 3);
    CHECK(one.gp_cap() == 1 && one.gp_data() == base.gp_data() + 2);
    slice<std::int64_t> tail = gp::gc::sub(s, 3);
    CHECK(gp::gc::len(tail) == 0 && gp::gc::cap(tail) == 2);
    CHECK(tail.gp_data() == base.gp_data() + 3);
    slice<std::int64_t> nil_sub = gp::gc::sub(nil_slice, 0, 0);
    CHECK(nil_sub == nullptr);
    slice<std::int64_t> empty_sub = gp::gc::sub(empty, 0, 0);
    CHECK(empty_sub != nullptr);

    // append: in place when the elements fit, a new array otherwise; other slices of the old
    // array do not see the new elements.
    slice<std::int64_t> inplace = gp::gc::append(s, std::int64_t(40));
    CHECK(inplace.gp_data() == base.gp_data());
    CHECK(gp::gc::len(inplace) == 4 && gp::gc::cap(inplace) == 5);
    CHECK(inplace[3] == 40);
    CHECK(gp::gc::len(s) == 3); // the old header is unchanged
    slice<std::int64_t> grew = gp::gc::append(inplace, std::int64_t(50));
    CHECK(gp::gc::len(grew) == 5 && gp::gc::cap(grew) == 5);
    CHECK(grew.gp_data() == base.gp_data());
    slice<std::int64_t> grew2 = keep_slice(gp::gc::append(grew, std::int64_t(60)));
    CHECK(gp::gc::len(grew2) == 6 && gp::gc::cap(grew2) == 10); // Go's growslice: 5 -> 10
    CHECK(grew2.gp_data() != base.gp_data());
    CHECK(grew2[5] == 60);

    // append with several elements at once, and append_slice.
    slice<std::int64_t> multi =
        keep_slice(gp::gc::append(keep_slice(gp::gc::make_slice<std::int64_t>(0)), std::int64_t(1),
                                  std::int64_t(2), std::int64_t(3)));
    CHECK(gp::gc::len(multi) == 3 && gp::gc::cap(multi) == 3);
    CHECK(multi[0] == 1 && multi[2] == 3);
    slice<std::int64_t> appended =
        keep_slice(gp::gc::append_slice(keep_slice(gp::gc::make_slice<std::int64_t>(0)),
                                        keep_slice(gp::gc::make_slice<std::int64_t>(20))));
    CHECK(gp::gc::len(appended) == 20 && gp::gc::cap(appended) == 20);

    // Growth is Go's runtime.growslice (real Go's numbers, not the compiler's inline append).
    check_caps<std::int64_t>("[]int64", {}, 40,
                             {{1, 1}, {2, 2}, {3, 4}, {5, 8}, {9, 16}, {17, 32}, {33, 64}});
    check_caps<gp::gc::string>("[]string", {}, 40,
                               {{1, 1}, {2, 2}, {3, 4}, {5, 8}, {9, 16}, {17, 32}, {33, 71}});
    check_caps<std::uint8_t>("[]byte", {}, 40, {{1, 8}, {9, 16}, {17, 32}, {33, 64}});
    check_caps<gp::gc::zero_field<0>>("[]struct{}", {}, 5,
                                      {{1, 1}, {2, 2}, {3, 3}, {4, 4}, {5, 5}});
    // A growslice from cap 0 rounds the request up to gc's size classes: 600 elements are
    // 4800 bytes, class 4864, 608 elements. The 848 of a slice grown one element at a time
    // comes from its larger old capacity; check_caps checks that path.
    CHECK(gp::gc::detail::grow_cap(600, 0, 8, true) == 608);
    CHECK(gp::gc::detail::grow_cap(1024, 0, 8, true) == 1024);
    check_caps<std::int64_t>("[]int64 to 600", {}, 600, {{600, 848}});
    // The 512-byte malloc-header boundary (64-byte pointer elements, noscan = false): 512 bytes
    // take no header, 576 do (the header makes it 632 bytes of class, 9 elements); grown one at
    // a time from cap 8, nextslicecap gives 16 and the header gives 17.
    CHECK(gp::gc::detail::grow_cap(8, 0, 64, false) == 8);
    CHECK(gp::gc::detail::grow_cap(9, 0, 64, false) == 9);
    CHECK(gp::gc::detail::grow_cap(9, 8, 64, false) == 17);
    // The end of the size-class table at 32760 bytes: at the boundary the header is included
    // (class 32768 - 8); above it the page path takes over.
    CHECK(gp::gc::detail::grow_cap(1, 0, 32760, false) == 1);
    CHECK(gp::gc::detail::grow_cap(2, 0, 32760, false) == 2);
    CHECK(gp::gc::detail::grow_cap(1, 0, 32768, false) == 1);
    // The page path: 40000 bytes of element data round up to 40960, 120000 to 122880; the
    // header is not part of this path (the two noscan values agree). Real Go: []int64 x5000
    // has cap 5120.
    CHECK(gp::gc::detail::grow_cap(5000, 0, 8, true) == 5120);
    CHECK(gp::gc::detail::grow_cap(5000, 0, 8, false) == 5120);
    CHECK(gp::gc::detail::grow_cap(3, 0, 40000, true) == 3);
    check_panic("growslice overflow", "runtime error: growslice: len out of range",
                "runtime.errorString", [] { (void)gp::gc::detail::grow_cap(-1, 0, 8, true); });

    // Overlapping copy and append: as if the source were read first.
    slice<std::int64_t> overlap = keep_slice(gp::gc::make_slice<std::int64_t>(5));
    for (std::int64_t i = 0; i < 5; i++) {
        overlap[i] = i;
    }
    std::int64_t moved = gp::gc::copy(gp::gc::sub(overlap, 1, 5), gp::gc::sub(overlap, 0, 4));
    CHECK(moved == 4);
    CHECK(overlap[0] == 0 && overlap[1] == 0 && overlap[2] == 1 && overlap[3] == 2 &&
          overlap[4] == 3);
    slice<std::int64_t> self = keep_slice(gp::gc::make_slice<std::int64_t>(4));
    self[0] = 7;
    self[1] = 8;
    self[2] = 9;
    self[3] = 10;
    slice<std::int64_t> doubled = keep_slice(gp::gc::append_slice(self, self));
    CHECK(gp::gc::len(doubled) == 8 && doubled[0] == 7 && doubled[4] == 7 && doubled[7] == 10);
    slice<std::int64_t> shifted =
        keep_slice(gp::gc::append_slice(gp::gc::sub(self, 0, 1), gp::gc::sub(self, 2, 4)));
    CHECK(gp::gc::len(shifted) == 3 && shifted[0] == 7 && shifted[1] == 9 && shifted[2] == 10);

    // copy([]byte, string), clear and slice_of.
    slice<std::uint8_t> bytes = keep_slice(gp::gc::make_slice<std::uint8_t>(4));
    CHECK(gp::gc::copy(bytes, gp::gc::lit("hi")) == 2);
    CHECK(bytes[0] == 'h' && bytes[1] == 'i' && bytes[2] == 0);
    gp::gc::clear(bytes);
    CHECK(bytes[0] == 0 && bytes[3] == 0);
    slice<std::uint8_t> appended_str = keep_slice(gp::gc::append_string(bytes, gp::gc::lit("go")));
    CHECK(gp::gc::len(appended_str) == 6 && appended_str[4] == 'g' && appended_str[5] == 'o');
    slice<std::uint8_t> from_string = keep_slice(gp::gc::to_bytes(gp::gc::lit("hey")));
    CHECK(gp::gc::len(from_string) == 3 && from_string[0] == 'h' && from_string[2] == 'y');
    string from_bytes = keep_string(gp::gc::to_string(from_string));
    CHECK(from_bytes == gp::gc::lit("hey"));
    string empty_string = gp::gc::to_string(gp::gc::make_slice<std::uint8_t>(0));
    CHECK(empty_string.gp_len() == 0);
    slice<std::int64_t> lit_slice = keep_slice(gp::gc::slice_of<std::int64_t>({4, 5, 6}));
    CHECK(gp::gc::len(lit_slice) == 3 && gp::gc::cap(lit_slice) == 3 && lit_slice[1] == 5);

    // make's checks.
    check_panic("make len -1", "runtime error: makeslice: len out of range", "runtime.errorString",
                [] { (void)gp::gc::make_slice<std::int64_t>(-1); });
    check_panic("make len huge", "runtime error: makeslice: len out of range",
                "runtime.errorString",
                [] { (void)gp::gc::make_slice<std::int64_t>(std::int64_t(1) << 50); });
    check_panic("make cap < len", "runtime error: makeslice: cap out of range",
                "runtime.errorString", [] { (void)gp::gc::make_slice<std::int64_t>(3, 2); });

    // A zero-size element takes no memory: any length, cap == len, and every element has the
    // same address.
    slice<gp::gc::zero_field<0>> zeros = gp::gc::make_slice<gp::gc::zero_field<0>>(1000);
    CHECK(gp::gc::len(zeros) == 1000 && gp::gc::cap(zeros) == 1000);
    CHECK(&zeros[0] == &zeros[999]);
    // Go's stride is 0, so every slicing form keeps the base pointer (never zerobase + l).
    CHECK(gp::gc::sub(zeros, 999, 1000).gp_data() == zeros.gp_data());
    CHECK(gp::gc::sub(zeros, 999, 1000).gp_len() == 1);
    CHECK(gp::gc::sub(zeros, 999, 1000).gp_cap() == 1);
    CHECK(gp::gc::sub(zeros, 999, 1000, 1000).gp_data() == zeros.gp_data());
    CHECK(gp::gc::sub(zeros, 999, 1000, 1000).gp_cap() == 1);
    CHECK(gp::gc::sub(zeros, 1000, 1000).gp_data() == zeros.gp_data());
    zeros = gp::gc::append(zeros, gp::gc::zero_field<0>{});
    CHECK(gp::gc::len(zeros) == 1001 && gp::gc::cap(zeros) == 1001);

    // --- arrays ---
    array<std::int64_t, 3> a{};
    a[0] = 1;
    a[1] = 2;
    a[2] = 3;
    CHECK(gp::gc::len(a) == 3 && gp::gc::cap(a) == 3);
    CHECK(a[1] == 2);
    check_panic("array index 3", "runtime error: index out of range [3] with length 3",
                "runtime.boundsError", [&] { (void)a[3]; });
    slice<std::int64_t> as = gp::gc::sub(a, 1, 3);
    CHECK(gp::gc::len(as) == 2 && gp::gc::cap(as) == 2);
    CHECK(as.gp_data() == a.gp_data() + 1 && as[0] == 2);
    CHECK(gp::gc::sub(a, 3, 3).gp_data() == a.gp_data());
    CHECK(gp::gc::sub(a, 3, 3).gp_cap() == 0);
    // A [5]T of a zero-size element: slicing keeps the array's base pointer, as Go does.
    array<gp::gc::zero_field<0>, 5> za{};
    slice<gp::gc::zero_field<0>> zas = gp::gc::sub(za, 3, 5);
    CHECK(zas.gp_len() == 2 && zas.gp_cap() == 2);
    CHECK(zas.gp_data() == za.gp_data());
    CHECK(gp::gc::sub(za, 3, 5, 5).gp_data() == za.gp_data());
    CHECK(gp::gc::sub(za, 3, 5, 5).gp_len() == 2 && gp::gc::sub(za, 3, 5, 5).gp_cap() == 2);
    array<std::int64_t, 0> az{};
    CHECK(gp::gc::len(az) == 0);
    slice<std::int64_t> azs = gp::gc::sub(az, 0, 0);
    CHECK(azs != nullptr && azs.gp_cap() == 0);
    check_panic("array[0] index 0", "runtime error: index out of range [0] with length 0",
                "runtime.boundsError", [&] { (void)az[0]; });
    check_panic("array slice hi", "runtime error: slice bounds out of range [:4] with length 3",
                "runtime.boundsError", [&] { (void)gp::gc::sub(a, 0, 4); });
    array<std::int64_t, 2> a2 = gp::gc::array_of<std::int64_t, 2>({7, 8});
    CHECK(a2[0] == 7 && a2[1] == 8);

    // --- interfaces ---
    eface nil_e;
    CHECK(nil_e.gp_nil());
    CHECK(nil_e == eface());
    eface e42 = keep_eface(to_eface<std::int64_t>(42));
    eface e42b = keep_eface(to_eface<std::int64_t>(42));
    eface e43 = keep_eface(to_eface<std::int64_t>(43));
    CHECK(!e42.gp_nil());
    CHECK(e42 == e42b);
    CHECK(e42 != e43);
    CHECK(e42 != nil_e);
    eface estr = keep_eface(to_eface(gp::gc::lit("hi")));
    eface estr2 = keep_eface(to_eface(gp::gc::lit("hi")));
    CHECK(estr == estr2);
    CHECK(estr != e42); // different dynamic types
    eface ezero = keep_eface(to_eface(gp::gc::zero_field<0>{}));
    CHECK(!ezero.gp_nil());
    CHECK(ezero == keep_eface(to_eface(gp::gc::zero_field<0>{})));
    std::int64_t n = 5;
    std::int64_t* p = &n;
    eface ep = keep_eface(to_eface(p));
    CHECK(ep == to_eface(p));
    CHECK(ep.gp_data() == static_cast<void*>(p)); // direct: the data word is the pointer
    std::int64_t* nullp = nullptr;
    eface enull = keep_eface(to_eface(nullp));
    CHECK(!enull.gp_nil()); // a typed nil pointer is not a nil interface
    CHECK(enull != ep);
    // An interface value in an interface keeps its dynamic type and data.
    CHECK(keep_eface(to_eface(e42)) == e42);
    // Slices are not comparable: Go panics.
    eface es1 = keep_eface(to_eface(keep_slice(gp::gc::make_slice<std::int64_t>(1))));
    eface es2 = keep_eface(to_eface(keep_slice(gp::gc::make_slice<std::int64_t>(1))));
    check_panic("uncomparable slice", "runtime error: comparing uncomparable type []T",
                "runtime.errorString", [&] { (void)(es1 == es2); });

    // The non-empty interface: the itab carries the dynamic type.
    gp::gc::itab* tab = keep_ptr(gp::gc::make_itab(gp::gc::type_of<std::int64_t>()));
    iface i42 = keep_iface(to_iface<std::int64_t>(tab, 42));
    iface i42b = keep_iface(to_iface<std::int64_t>(tab, 42));
    iface i43 = keep_iface(to_iface<std::int64_t>(tab, 43));
    iface nil_i;
    CHECK(!i42.gp_nil());
    CHECK(i42 == i42b);
    CHECK(i42 != i43);
    CHECK(i42 != nil_i);
    CHECK(keep_iface(to_iface<std::int64_t>(tab, 42)) == keep_iface(to_iface(tab, e42)));
    // A struct's equality is fieldwise, and an interface boxes it.
    point pt{1, 2};
    point pt2{1, 2};
    point pt3{1, 3};
    CHECK(gp::gc::equal(pt, pt2));
    CHECK(!gp::gc::equal(pt, pt3));
    CHECK(keep_eface(to_eface(pt)) == keep_eface(to_eface(pt2)));
    CHECK(keep_eface(to_eface(pt)) != keep_eface(to_eface(pt3)));
    with_zero wz{};
    wz.A = 9;
    with_zero wz2{};
    wz2.A = 9;
    CHECK(gp::gc::equal(wz, wz2)); // the zero-size field and the padding do not matter

    // --- funcs ---
    auto* closure = keep_ptr(gp::gc::new_<add_closure>(add_closure{{add_call}, 10}));
    func<add_sig> f = func<add_sig>::from_closure(closure);
    CHECK(!f.gp_nil());
    CHECK(f(1, 2) == 13);
    CHECK(f.gp_closure() == static_cast<void*>(closure));
    func<add_sig> nil_f;
    CHECK(nil_f == nullptr);
    check_panic("nil func call", "runtime error: invalid memory address or nil pointer dereference",
                "runtime.errorString", [&] { (void)nil_f(1, 2); });

    // --- maps and channels (placeholder tables) ---
    map<std::int64_t, std::int64_t> nil_m;
    CHECK(nil_m == nullptr);
    CHECK(gp::gc::len(nil_m) == 0);
    map<std::int64_t, std::int64_t> m = gp::gc::make_map<std::int64_t, std::int64_t>();
    keep_ptr(m.gp_table());
    CHECK(m != nullptr);
    CHECK(gp::gc::len(m) == 0);
    CHECK(m.gp_table()->gp_key_size == 8 && m.gp_table()->gp_elem_size == 8);
    CHECK(m.gp_table()->gp_key_ptrs == false && m.gp_table()->gp_elem_ptrs == false);
    gp::gc::chan<std::int64_t> nil_c;
    CHECK(nil_c == nullptr && gp::gc::len(nil_c) == 0 && gp::gc::cap(nil_c) == 0);
    gp::gc::chan<std::int64_t> c = gp::gc::make_chan<std::int64_t>(4);
    keep_ptr(c.gp_core());
    keep_ptr(c.gp_core()->gp_buf);
    CHECK(c != nullptr);
    CHECK(gp::gc::cap(c) == 4 && gp::gc::len(c) == 0);
    CHECK(c.gp_core()->gp_elem_size == 8 && c.gp_core()->gp_buf != nullptr);
    CHECK(c == c);
    {
        auto other = gp::gc::make_chan<std::int64_t>(4);
        keep_ptr(other.gp_core());
        keep_ptr(other.gp_core()->gp_buf);
        CHECK(c != other);
    }
    check_panic("makechan negative", "makechan: size out of range", "runtime.plainError",
                [] { (void)gp::gc::make_chan<std::int64_t>(-1); });

    // --- new_, strings, arithmetic ---
    std::int64_t* fresh = keep_ptr(gp::gc::new_<std::int64_t>());
    CHECK(*fresh == 0);
    std::int64_t* five = keep_ptr(gp::gc::new_<std::int64_t>(std::int64_t(5)));
    CHECK(*five == 5);

    string str = gp::gc::lit("hello");
    string mid = gp::gc::sub(str, 1, 3);
    CHECK(mid.gp_view() == "el");
    CHECK(mid.gp_data() == str.gp_data() + 1);
    string empty_str = gp::gc::sub(str, 5, 5);
    CHECK(empty_str.gp_len() == 0);
    CHECK(empty_str.gp_data() == str.gp_data()); // Go keeps the base pointer for an empty result
    string empty_mid = gp::gc::sub(str, 2, 2);
    CHECK(empty_mid.gp_len() == 0 && empty_mid.gp_data() == str.gp_data());
    check_panic("string slice hi", "runtime error: slice bounds out of range [:6] with length 5",
                "runtime.boundsError", [&] { (void)gp::gc::sub(str, 0, 6); });

    CHECK(gp::gc::shl(1, 63) == static_cast<std::int64_t>(std::uint64_t(1) << 63));
    CHECK(gp::gc::shl(1, 64) == 0);
    CHECK(gp::gc::shr(-8, 1) == -4);
    CHECK(gp::gc::shr(-1, 64) == -1);
    CHECK(gp::gc::div(std::int64_t(-7), 2) == -3); // Go truncates toward zero
    CHECK(gp::gc::rem(std::int64_t(-7), 2) == -1);
    CHECK(gp::gc::div(INT64_MIN, -1) == INT64_MIN);
    CHECK(gp::gc::rem(INT64_MIN, -1) == 0);
    check_panic("divide by zero", "runtime error: integer divide by zero", "runtime.errorString",
                [] { (void)gp::gc::div(1, 0); });
    check_panic("remainder by zero", "runtime error: integer divide by zero", "runtime.errorString",
                [] { (void)gp::gc::rem(1, 0); });
    check_panic("negative shift", "runtime error: negative shift amount", "runtime.errorString",
                [] { (void)gp::gc::shl(1, -1); });

    // --- nil dereference ---
    std::int64_t* np = nullptr;
    check_panic("nil deref", "runtime error: invalid memory address or nil pointer dereference",
                "runtime.errorString", [&] { (void)gp::gc::deref(np); });

    if (failures != 0) {
        std::fprintf(stderr, "%d of %d checks failed\n", failures, checks);
        return 1;
    }
    std::printf("ok: %d checks\n", checks);
    return 0;
}
