// The unit tests of gp::array (gocpp/array.hpp): layout, zero values, value semantics, the
// index check, ==, [0]T, literals, the conversions from slices, slicing a boxed array, the range
// loops and elem_ptr.
// runtime/cxx_test.go compiles this file with the runtime's .cpp files, with g++ and clang++,
// at -O0, at -O2 and with ASan + UBSan (LeakSanitizer included), and runs it like
// test/unit.cpp. It prints "ok: N checks" and exits 0, or prints each failed check and exits 1.
// The Go behavior it checks is in runtime/testdata/diff/main.go (arrayCase) too, where real Go
// runs it.
#include <gocpp/rt.hpp>
#include <gocpp/os.hpp>

#include <cmath>
#include <compare>
#include <concepts>
#include <cstdint>
#include <cstring>
#include <limits>
#include <memory>
#include <new>
#include <string>
#include <type_traits>
#include <utility>
#include <vector>

// A user's package with Go functions whose names are also the names of runtime functions that
// array.hpp calls with arguments of a user type (gp::len of a slice of P, gp::sub of a pointer
// to an array of P). The runtime's calls are qualified, so argument-dependent lookup never
// picks them (test_adl).
namespace gpkg_adl {

struct P {
    int64_t v = 0;
    friend bool operator==(const P&, const P&) = default;
};

int64_t hijacked = 0;

// Not templates, so an unqualified call would pick them over the runtime's templates.
int64_t len(const gp::slice<P>&) {
    hijacked++;
    return 0;
}
gp::slice<P> sub(gp::ptr<gp::array<P, 3>>, int64_t, int64_t) {
    hijacked++;
    return nullptr;
}

} // namespace gpkg_adl

namespace {

int checks = 0;
int failures = 0;

std::string show(int64_t v) {
    return std::to_string(static_cast<long long>(v));
}
std::string show(const std::string& v) {
    return '"' + v + '"';
}

void report(bool ok, const char* file, int line, const std::string& what) {
    checks++;
    if (!ok) {
        failures++;
        std::string msg = std::string(file) + ":" + std::to_string(line) + ": " + what + "\n";
        gp::os::write(gp::os::stderr_fd, msg.data(), msg.size());
    }
}

#define CHECK(...) report((__VA_ARGS__), __FILE__, __LINE__, "CHECK(" #__VA_ARGS__ ") failed")

#define CHECK_EQ(got, want)                                                                        \
    do {                                                                                           \
        auto gp_got = (got);                                                                       \
        decltype(gp_got) gp_want = (want);                                                         \
        report(gp_got == gp_want, __FILE__, __LINE__,                                              \
               #got " = " + show(gp_got) + ", want " + show(gp_want));                             \
    } while (0)

// panic_text runs f and returns the text of the panic it throws, or "<no panic>".
template <typename F>
std::string panic_text(F f) {
    try {
        f();
    } catch (const gp::panic_error& e) {
        return std::string(e.gp_text.gp_view());
    }
    return "<no panic>";
}

using ints = gp::slice<int64_t>;
template <int64_t N>
using iarr = gp::array<int64_t, N>;

template <typename T, int64_t N>
bool same(const gp::array<T, N>& a, std::vector<T> want) {
    if (want.size() != static_cast<std::size_t>(N)) {
        return false;
    }
    for (int64_t i = 0; i < N; i++) {
        if (!(a[i] == want[static_cast<std::size_t>(i)])) {
            return false;
        }
    }
    return true;
}

bool same(const ints& s, std::vector<int64_t> want) {
    std::vector<int64_t> v;
    for (int64_t i = 0; i < gp::len(s); i++) {
        v.push_back(s[i]);
    }
    return v == want;
}

// --- layout and the type's rules ---

struct Plain {
    int64_t a;
    double b;
    bool c;
};

// [N]T is exactly N elements of T, with T's alignment; [0]T is an empty struct.
static_assert(sizeof(iarr<3>) == 24 && alignof(iarr<3>) == 8);
static_assert(sizeof(iarr<1>) == 8);
static_assert(sizeof(gp::array<bool, 5>) == 5 && alignof(gp::array<bool, 5>) == 1);
static_assert(sizeof(gp::array<int16_t, 3>) == 6 && alignof(gp::array<int16_t, 3>) == 2);
static_assert(sizeof(gp::array<gp::array<int16_t, 3>, 2>) == 12);
static_assert(sizeof(gp::array<gp::string, 2>) == 64);
static_assert(sizeof(gp::array<gp::ptr<int64_t>, 4>) == 64);
static_assert(sizeof(gp::array<gp::slice<int64_t>, 2>) == 80);
static_assert(sizeof(gp::array<Plain, 3>) == 3 * sizeof(Plain));
static_assert(sizeof(gp::array<int64_t, 0>) == 1 && alignof(gp::array<int64_t, 0>) == 1);
static_assert(sizeof(gp::array<gp::string, 0>) == 1 && alignof(gp::array<gp::string, 0>) == 1);
static_assert(std::is_empty_v<gp::array<gp::string, 0>>);
static_assert(sizeof(gp::array<gp::array<int64_t, 0>, 4>) == 4); // [4][0]int: 4 empty structs

// An aggregate, trivially copyable when T is (copies are memcpy), with no implicit conversion
// to bool and no ordering (Go has neither).
static_assert(std::is_aggregate_v<iarr<3>> && std::is_aggregate_v<iarr<0>>);
static_assert(std::is_trivially_copyable_v<iarr<3>> && std::is_trivially_destructible_v<iarr<3>>);
static_assert(std::is_trivially_copyable_v<iarr<0>>);
static_assert(!std::is_convertible_v<iarr<3>, bool>);
static_assert(!std::three_way_comparable<iarr<3>>);
template <typename T>
concept ordered = requires(T a) { a < a; };
static_assert(!ordered<iarr<3>> && !ordered<iarr<0>>);
static_assert(std::is_nothrow_move_constructible_v<gp::array<gp::string, 2>>);

// == only when the elements are comparable: arrays of slices are not, as in Go.
static_assert(std::equality_comparable<iarr<3>>);
static_assert(std::equality_comparable<gp::array<double, 2>>);
static_assert(std::equality_comparable<gp::array<gp::string, 2>>);
static_assert(std::equality_comparable<gp::array<gp::ptr<int64_t>, 2>>);
static_assert(std::equality_comparable<gp::array<iarr<2>, 2>>);
static_assert(std::equality_comparable<iarr<0>>);
static_assert(!std::equality_comparable<gp::array<ints, 2>>);
static_assert(!std::equality_comparable<gp::array<ints, 0>>);
static_assert(!std::equality_comparable<gp::array<gp::array<ints, 1>, 2>>);
static_assert(!std::equality_comparable<gp::array<Plain, 2>>); // Plain has no ==

// a[i] gives a writable element, and a const one for a const array (an array is a value).
static_assert(std::is_same_v<decltype(std::declval<iarr<3>&>()[0]), int64_t&>);
static_assert(std::is_same_v<decltype(std::declval<const iarr<3>&>()[0]), const int64_t&>);
static_assert(std::is_same_v<decltype(std::declval<iarr<0>&>()[0]), int64_t&>);
static_assert(std::is_same_v<decltype(std::declval<const iarr<0>&>()[0]), const int64_t&>);

// len and cap are constants, also of a pointer to an array.
static_assert(gp::len(iarr<3>{}) == 3 && gp::cap(iarr<3>{}) == 3 && gp::len(iarr<0>{}) == 0);
static_assert(std::is_same_v<decltype(gp::len(std::declval<iarr<3>&>())), int64_t>);

// Constant initialization: package-level arrays with constant values need no dynamic
// initializer (constinit), and the literal forms are constant expressions.
constinit iarr<3> global_zero;
constinit iarr<3> global_lit{1, 2, 3};
constinit iarr<5> global_part{7, 8};
constinit iarr<5> global_keyed = gp::array_keyed<int64_t, 5>({{4, 40}, {1, 10}});
constinit gp::array<gp::array<int32_t, 2>, 2> global_nested{gp::array<int32_t, 2>{1, 2},
                                                            gp::array<int32_t, 2>{3, 4}};
constinit gp::array<double, 0> global_empty;
constinit gp::array<gp::string, 2> global_strs{gp::string(), gp::string()};
static_assert(iarr<3>{1, 2, 3} == iarr<3>{1, 2, 3});
static_assert(iarr<3>{1, 2, 3} != iarr<3>{1, 2, 4});
static_assert(gp::array_keyed<int64_t, 3>({{2, 5}}) == iarr<3>{0, 0, 5});
static_assert(iarr<0>{} == iarr<0>{});

// A package-level array that is sliced (by the static owner).
iarr<4> global_arr{1, 2, 3, 4};

void test_zero_and_literals() {
    // A declared array is all zero, without an initializer too: build one in dirty memory.
    alignas(iarr<4>) unsigned char buf[sizeof(iarr<4>)];
    std::memset(buf, 0xff, sizeof buf);
    auto* a = new (buf) iarr<4>;
    CHECK(same(*a, {0, 0, 0, 0}));
    alignas(gp::array<Plain, 2>) unsigned char pbuf[sizeof(gp::array<Plain, 2>)];
    std::memset(pbuf, 0xff, sizeof pbuf);
    auto* pa = new (pbuf) gp::array<Plain, 2>;
    CHECK(pa->gp_elems[1].a == 0 && pa->gp_elems[1].b == 0.0 && !pa->gp_elems[1].c);
    gp::array<gp::string, 2> s;
    CHECK(s[0] == ""_gs && s[1] == ""_gs);
    gp::array<gp::ptr<int64_t>, 2> ps;
    CHECK(ps[0] == nullptr && ps[1] == nullptr);
    gp::array<ints, 2> ss;
    CHECK(ss[1] == nullptr);
    gp::array<double, 2> ds;
    CHECK(ds[0] == 0.0 && !std::signbit(ds[1]));

    CHECK(same(global_zero, {0, 0, 0}));
    CHECK(same(global_lit, {1, 2, 3}));
    CHECK(same(global_part, {7, 8, 0, 0, 0}));
    CHECK(same(global_keyed, {0, 10, 0, 0, 40}));
    CHECK(global_nested[1][0] == 3 && global_nested[0][1] == 2);
    CHECK(global_strs[1] == ""_gs);
    CHECK_EQ(gp::len(global_empty), INT64_C(0));

    // Literals: positional, partial, nested, with zero elements written as T().
    iarr<3> l{1, 2, 3};
    CHECK(same(l, {1, 2, 3}));
    gp::array<gp::string, 3> ls{"a"_gs, gp::string(), "c"_gs};
    CHECK(ls[0] == "a"_gs && ls[1] == ""_gs && ls[2] == "c"_gs);
    gp::array<iarr<2>, 2> nested{iarr<2>{1, 2}, iarr<2>{3, 4}};
    CHECK(nested[1][1] == 4 && nested[0][0] == 1);
    gp::array<gp::array<gp::string, 1>, 2> ns{gp::array<gp::string, 1>{"x"_gs}};
    CHECK(ns[0][0] == "x"_gs && ns[1][0] == ""_gs);

    // Keyed literals: stored at their keys, evaluated in source order.
    std::vector<int> order;
    auto at = [&](int n, int64_t v) {
        order.push_back(n);
        return v;
    };
    iarr<5> k = gp::array_keyed<int64_t, 5>({{3, at(1, 30)}, {0, at(2, 1)}, {4, at(3, 7)}});
    CHECK(same(k, {1, 0, 0, 30, 7}));
    CHECK((order == std::vector<int>{1, 2, 3}));
    gp::array<gp::string, 3> ks = gp::array_keyed<gp::string, 3>({{2, "z"_gs}});
    CHECK(ks[0] == ""_gs && ks[2] == "z"_gs);
    CHECK_EQ(panic_text([] { gp::array_keyed<int64_t, 2>({{2, 1}}); }),
             std::string("runtime error: index out of range [2] with length 2"));
    CHECK_EQ(panic_text([] { gp::array_keyed<int64_t, 0>({{0, 1}}); }),
             std::string("runtime error: index out of range [0] with length 0"));
    CHECK(gp::array_keyed<int64_t, 0>({}) == iarr<0>{});
}

// --- value semantics ---

iarr<3> modify(iarr<3> a) {
    a[0] = 100;
    return a;
}

void test_values() {
    iarr<3> a{1, 2, 3};
    iarr<3> b = a; // a copy
    b[0] = 9;
    CHECK(same(a, {1, 2, 3}) && same(b, {9, 2, 3}));
    iarr<3> c = modify(a); // passed by value
    CHECK(same(a, {1, 2, 3}) && same(c, {100, 2, 3}));
    c = a; // assignment copies
    a[2] = 30;
    CHECK(same(c, {1, 2, 3}));

    // Nested arrays copy deeply.
    gp::array<iarr<2>, 2> n{iarr<2>{1, 2}, iarr<2>{3, 4}};
    gp::array<iarr<2>, 2> m = n;
    m[1][0] = 33;
    CHECK(n[1][0] == 3 && m[1][0] == 33);
    iarr<2> row = n[0]; // an element that is an array is a copy too
    row[0] = 11;
    CHECK(n[0][0] == 1);

    // Arrays of strings: copies of the elements (strings are immutable, so they can share).
    gp::array<gp::string, 2> s{gp::string::gp_copy("heap"), "lit"_gs};
    gp::array<gp::string, 2> t = s;
    t[0] = "other"_gs;
    CHECK(s[0] == "heap"_gs && t[0] == "other"_gs && t[1] == "lit"_gs);

    // Arrays of pointers copy the pointers: both arrays point at the same objects.
    gp::array<gp::ptr<int64_t>, 2> p{gp::new_<int64_t>(5), nullptr};
    gp::array<gp::ptr<int64_t>, 2> q = p;
    *q[0] = 6;
    q[1] = gp::new_<int64_t>(7);
    CHECK(*p[0] == 6 && p[1] == nullptr && *q[1] == 7);

    // Arrays of slices copy the slice headers: both see the same backing array.
    gp::array<ints, 1> sa{gp::slice_of<int64_t>({1, 2})};
    gp::array<ints, 1> sb = sa;
    sb[0][0] = 10;
    CHECK(sa[0][0] == 10);
    sb[0] = gp::append(sb[0], INT64_C(3));
    CHECK(gp::len(sa[0]) == 2 && gp::len(sb[0]) == 3);

    // Moving an array moves its elements; the source keeps zero or moved-from elements.
    gp::array<ints, 1> moved = std::move(sb);
    CHECK(gp::len(moved[0]) == 3);
    // Self-assignment.
    iarr<3>& ar = a;
    a = ar;
    CHECK(same(a, {1, 2, 30}));
}

// --- index ---

void test_index() {
    iarr<3> a{10, 20, 30};
    const iarr<3>& ca = a;
    CHECK_EQ(a[0], INT64_C(10));
    CHECK_EQ(a[uint8_t(2)], INT64_C(30));
    a[int8_t(1)] = 21;
    CHECK_EQ(ca[uint64_t(1)], INT64_C(21));
    std::string p = "runtime error: index out of range ";
    CHECK_EQ(panic_text([&] { a[3]; }), p + "[3] with length 3");
    CHECK_EQ(panic_text([&] { ca[3]; }), p + "[3] with length 3");
    CHECK_EQ(panic_text([&] { a[-1]; }), p + "[-1]");
    CHECK_EQ(panic_text([&] { a[int8_t(-128)]; }), p + "[-128]");
    CHECK_EQ(panic_text([&] { a[uint8_t(200)]; }), p + "[200] with length 3");
    CHECK_EQ(panic_text([&] { a[UINT64_MAX]; }), p + "[18446744073709551615] with length 3");
    CHECK_EQ(panic_text([&] { a[INT64_MIN]; }), p + "[-9223372036854775808]");
    iarr<0> z;
    const iarr<0>& cz = z;
    CHECK_EQ(panic_text([&] { z[0]; }), p + "[0] with length 0");
    CHECK_EQ(panic_text([&] { cz[uint32_t(7)]; }), p + "[7] with length 0");
    CHECK_EQ(panic_text([&] { z[-2]; }), p + "[-2]");
    gp::array<iarr<2>, 2> n{};
    CHECK_EQ(panic_text([&] { n[1][2]; }), p + "[2] with length 2");
    CHECK_EQ(panic_text([&] { n[2][0]; }), p + "[2] with length 2");
}

// --- == ---

void test_equal() {
    CHECK(iarr<3>{1, 2, 3} == iarr<3>{1, 2, 3});
    CHECK(iarr<3>{1, 2, 3} != iarr<3>{1, 2, 4});
    CHECK(iarr<3>{} == iarr<3>{0, 0, 0});
    double nan = std::numeric_limits<double>::quiet_NaN();
    gp::array<double, 2> f{1.5, nan};
    gp::array<double, 2> g = f; // a copy of the same bits
    CHECK(!(f == g) && f != g); // NaN != NaN, as in Go
    CHECK(!(f == f));
    CHECK((gp::array<double, 2>{0.0, 1} == gp::array<double, 2>{-0.0, 1})); // +0 == -0
    gp::array<float, 1> nf{std::numeric_limits<float>::quiet_NaN()};
    CHECK(nf != nf);
    gp::array<iarr<2>, 2> n1{iarr<2>{1, 2}, iarr<2>{3, 4}};
    gp::array<iarr<2>, 2> n2 = n1;
    CHECK(n1 == n2);
    n2[1][1] = 5;
    CHECK(n1 != n2);
    gp::array<gp::array<double, 1>, 1> nn{gp::array<double, 1>{nan}};
    CHECK(nn != nn);
    gp::array<gp::string, 2> s1{"a"_gs, gp::string::gp_copy("b")};
    gp::array<gp::string, 2> s2{gp::string::gp_copy("a"), "b"_gs};
    CHECK(s1 == s2);
    s2[1] = "c"_gs;
    CHECK(s1 != s2);
    // Pointers compare by address.
    gp::ptr<int64_t> x = gp::new_<int64_t>(1), y = gp::new_<int64_t>(1);
    CHECK((gp::array<gp::ptr<int64_t>, 2>{x, nullptr} == gp::array<gp::ptr<int64_t>, 2>{x}));
    CHECK((gp::array<gp::ptr<int64_t>, 1>{x} != gp::array<gp::ptr<int64_t>, 1>{y}));
    CHECK(iarr<0>{} == iarr<0>{});
    CHECK(!(gp::array<double, 0>{} != gp::array<double, 0>{}));
    CHECK(gp::array<gpkg_adl::P, 2>{gpkg_adl::P{1}} == gp::array<gpkg_adl::P, 2>{gpkg_adl::P{1}});
}

// --- lifetimes ---

// Tracked counts its live objects and catches a double destruction.
int64_t live = 0;
int64_t bad_destroys = 0;

struct Tracked {
    int64_t v = 0;
    int64_t magic = 0x5eed;
    Tracked() { live++; }
    Tracked(int64_t x) : v(x) { live++; }
    Tracked(const Tracked& o) : v(o.v) { live++; }
    Tracked& operator=(const Tracked& o) {
        v = o.v;
        return *this;
    }
    ~Tracked() {
        if (magic != 0x5eed) {
            bad_destroys++;
        }
        magic = 0;
        live--;
    }
};

void test_lifetimes() {
    {
        gp::array<Tracked, 3> a;
        CHECK_EQ(live, INT64_C(3));
        gp::array<Tracked, 3> b = a;
        CHECK_EQ(live, INT64_C(6));
        b = gp::array<Tracked, 3>{Tracked(1), Tracked(2)};
        CHECK_EQ(live, INT64_C(6));
        CHECK(b[1].v == 2 && b[2].v == 0);
        gp::array<gp::array<Tracked, 2>, 2> n;
        CHECK_EQ(live, INT64_C(10));
        gp::ptr<gp::array<Tracked, 4>> boxed = gp::new_<gp::array<Tracked, 4>>();
        CHECK_EQ(live, INT64_C(14));
        gp::slice<Tracked> s = gp::sub(boxed, 1, 3);
        boxed = nullptr; // the slice keeps the box alive
        CHECK_EQ(live, INT64_C(14));
        s = nullptr;
        CHECK_EQ(live, INT64_C(10));
        gp::array<Tracked, 2> conv =
            gp::to_array<2>(gp::slice_of<Tracked>({Tracked(5), Tracked(6)}));
        (void)conv;
    }
    CHECK_EQ(live, INT64_C(0));
    CHECK_EQ(bad_destroys, INT64_C(0));

    // Elements that own things free them once, with the array (LeakSanitizer checks the rest).
    gp::ptr<int64_t> target = gp::new_<int64_t>(7);
    gp::weak_ptr<int64_t> wt = gp::weak_make(target);
    {
        gp::array<gp::ptr<int64_t>, 3> ps{target, target};
        target = nullptr;
        gp::array<gp::ptr<int64_t>, 3> copy = ps;
        ps = gp::array<gp::ptr<int64_t>, 3>{};
        CHECK(gp::weak_value(wt) != nullptr);
        copy[0] = nullptr;
        CHECK(gp::weak_value(wt) != nullptr);
    }
    CHECK(gp::weak_value(wt) == nullptr);

    ints inner = gp::slice_of<int64_t>({1});
    gp::weak_ptr<int64_t> wi = gp::weak_make(gp::ptr<int64_t>(inner.gp_owner(), &inner[0]));
    {
        gp::array<ints, 2> outer{inner, inner};
        inner = nullptr;
        gp::array<gp::array<ints, 2>, 2> nested{outer, outer};
        CHECK(gp::weak_value(wi) != nullptr);
    }
    CHECK(gp::weak_value(wi) == nullptr);

    gp::array<gp::string, 2> strs{gp::string::gp_copy("heap"), gp::string::gp_copy("more")};
    gp::array<gp::string, 2> strs2 = strs;
    strs = gp::array<gp::string, 2>{};
    CHECK(strs2[0] == "heap"_gs && strs[0] == ""_gs);
}

// A slice of Tracked with n zero elements, for the conversions.
gp::slice<Tracked> tracked_slice(int64_t n) {
    return gp::make_slice<Tracked>(n);
}

// --- freeing long chains (through gp::detail::release) ---

int64_t freed = 0;

struct ALink {
    gp::array<gp::ptr<ALink>, 1> next;
    ~ALink() { freed++; }
};
// Links held by value inside an array inside a boxed array.
struct BLink {
    gp::array<gp::ptr<gp::array<BLink, 1>>, 1> next;
    ~BLink() { freed++; }
};

bool nothing_pending() {
    return gp::detail::release_tls.gp_depth == 0 && gp::detail::release_tls.gp_pending == nullptr;
}

void test_chains() {
    const int64_t n = 200'000;
    freed = 0;
    {
        gp::ptr<ALink> head;
        for (int64_t i = 0; i < n; i++) {
            gp::ptr<ALink> l = gp::new_<ALink>();
            l->next[0] = std::move(head);
            head = std::move(l);
        }
    }
    CHECK_EQ(freed, n);
    CHECK(nothing_pending());

    freed = 0;
    {
        gp::ptr<gp::array<BLink, 1>> top;
        for (int64_t i = 0; i < n; i++) {
            gp::ptr<gp::array<BLink, 1>> b = gp::new_<gp::array<BLink, 1>>();
            (*b)[0].next[0] = std::move(top);
            top = std::move(b);
        }
    }
    CHECK_EQ(freed, n);
    CHECK(nothing_pending());
}

// --- conversions from slices ---

void test_convert() {
    std::string msg = "runtime error: cannot convert slice with length ";

    // [N]T(s): a copy of s[0:N].
    ints s = gp::slice_of<int64_t>({1, 2, 3, 4});
    iarr<3> a = gp::to_array<3>(s);
    CHECK(same(a, {1, 2, 3}));
    a[0] = 10;
    s[1] = 20;
    CHECK(s[0] == 1 && a[1] == 2); // independent
    CHECK(same(gp::to_array<4>(s), {1, 20, 3, 4}));
    CHECK(gp::to_array<0>(ints()) == iarr<0>{}); // a nil slice converts to [0]T
    CHECK(gp::to_array<0>(s) == iarr<0>{});
    CHECK(same(gp::to_array<2>(gp::sub(s, 2)), {3, 4}));
    CHECK_EQ(panic_text([&] { gp::to_array<5>(s); }),
             msg + "4 to array or pointer to array with length 5");
    CHECK_EQ(panic_text([] { gp::to_array<1>(ints()); }),
             msg + "0 to array or pointer to array with length 1");
    gp::slice<gp::string> ss = gp::slice_of<gp::string>({gp::string::gp_copy("x"), "y"_gs});
    gp::array<gp::string, 2> sa = gp::to_array<2>(ss);
    ss = nullptr;
    CHECK(sa[0] == "x"_gs && sa[1] == "y"_gs);
    {
        gp::slice<Tracked> ts = tracked_slice(3);
        CHECK_EQ(live, INT64_C(3));
        gp::array<Tracked, 2> ta = gp::to_array<2>(ts);
        CHECK_EQ(live, INT64_C(5));
        (void)ta;
    }
    CHECK_EQ(live, INT64_C(0));

    // (*[N]T)(s): shares s's array; writes are seen both ways; it keeps the array alive.
    ints t = gp::slice_of<int64_t>({1, 2, 3, 4});
    gp::ptr<iarr<2>> p = gp::to_array_ptr<2>(gp::sub(t, 1));
    CHECK(same(*p, {2, 3}));
    (*p)[0] = 22;
    t[2] = 33;
    CHECK(t[1] == 22 && (*p)[1] == 33);
    CHECK(p.gp_get()->gp_elems.data() == &t[1]);
    gp::weak_ptr<iarr<2>> wp = gp::weak_make(p);
    t = nullptr;
    CHECK(same(*p, {22, 33}));
    p = nullptr;
    CHECK(gp::weak_value(wp) == nullptr); // freed with the last pointer
    CHECK_EQ(panic_text([] { gp::to_array_ptr<5>(gp::slice_of<int64_t>({1, 2})); }),
             msg + "2 to array or pointer to array with length 5");
    CHECK_EQ(panic_text([] { gp::to_array_ptr<1>(ints()); }),
             msg + "0 to array or pointer to array with length 1");

    // (*[0]T)(s): nil for a nil s, non-nil (with an owner) for an empty one, as in Go.
    CHECK(gp::to_array_ptr<0>(ints()) == nullptr);
    gp::ptr<iarr<0>> e = gp::to_array_ptr<0>(gp::slice_of<int64_t>({}));
    CHECK(e != nullptr);
    CHECK(gp::weak_value(gp::weak_make(e)) != nullptr); // it has an owner: weak works
    ints four = gp::slice_of<int64_t>({1, 2, 3, 4});
    gp::ptr<iarr<0>> z = gp::to_array_ptr<0>(gp::sub(four, 4));
    CHECK(z != nullptr && z.gp_shared().use_count() > 1);
    // A slice of a package-level array: the pointer shares the static owner.
    ints g = gp::sub(gp::static_ptr(global_arr), 0);
    gp::ptr<iarr<4>> gpa = gp::to_array_ptr<4>(g);
    CHECK(gpa.gp_get() == &global_arr);
    gp::weak_ptr<iarr<4>> wg = gp::weak_make(gpa);
    g = nullptr;
    gpa = nullptr;
    CHECK(gp::weak_value(wg) != nullptr);
}

// views_alias: two pointer views of different lengths over one slice. No gp::array object
// exists at that address (the store is a T[]), so the writes through p2 and the reads through
// p3 alias only because every gocpp compile uses -fno-strict-aliasing: without it, g++ -O2
// treats gp::array<T, 2> and gp::array<T, 3> accesses as non-aliasing and reuses the first
// read (after[0] == 0). Not inlined, so the compiler sees only the pointers.
[[gnu::noinline]] std::pair<iarr<3>, iarr<3>> views_alias(const ints& s) {
    gp::ptr<iarr<2>> p2 = gp::to_array_ptr<2>(s);
    gp::ptr<iarr<3>> p3 = gp::to_array_ptr<3>(s);
    iarr<3> before = *p3;
    *p2 = iarr<2>{9, 9};
    iarr<3> after = *p3;
    return {before, after};
}

void test_views_alias() {
    ints s = gp::make_slice<int64_t>(3);
    s[2] = 3;
    auto [before, after] = views_alias(s);
    CHECK(same(before, {0, 0, 3}));
    CHECK(same(after, {9, 9, 3}));
    CHECK(s[0] == 9 && s[1] == 9);
}

// --- slicing a boxed array ---

struct Holder {
    int64_t n = 0;
    iarr<3> arr;
};

void test_sub() {
    // A boxed local: the box is a gp::ptr to the array.
    gp::ptr<iarr<5>> box = gp::new_<iarr<5>>(iarr<5>{1, 2, 3, 4, 5});
    ints s = gp::sub(box, 1, 3);
    CHECK(gp::len(s) == 2 && gp::cap(s) == 4 && same(s, {2, 3}));
    s[0] = 20;
    (*box)[2] = 30;
    CHECK((*box)[1] == 20 && s[1] == 30); // writes are seen both ways
    s = gp::append(s, INT64_C(40));       // fits: writes into the array, as in Go
    CHECK((*box)[3] == 40);
    CHECK(same(gp::sub(box, 0), {1, 20, 30, 40, 5}));
    CHECK(same(gp::sub(box, 3), {40, 5}));
    ints s3 = gp::sub(box, 1, 2, 3);
    CHECK(gp::len(s3) == 1 && gp::cap(s3) == 2);
    s3 = gp::append(s3, INT64_C(-3)); // fits up to max
    s3 = gp::append(s3, INT64_C(-4)); // doesn't: a new array
    CHECK((*box)[2] == -3 && (*box)[3] == 40);
    CHECK(gp::cap(gp::sub(box, 5)) == 0 && gp::sub(box, 5).gp_data() == box->gp_elems.data());
    CHECK(gp::sub(box, 2, 2).gp_data() == box->gp_elems.data() + 2);
    CHECK(gp::sub(box, uint8_t(1), int16_t(4), uint64_t(5)) != nullptr);

    // The slice keeps the box alive after the variable's pointer goes.
    gp::weak_ptr<iarr<5>> wb = gp::weak_make(box);
    ints keep = gp::sub(box, 0, 2);
    box = nullptr;
    CHECK(gp::weak_value(wb) != nullptr);
    keep[0] = 100;
    CHECK(same(keep, {100, 20}));
    s = nullptr;
    s3 = nullptr;
    keep = nullptr;
    CHECK(gp::weak_value(wb) == nullptr);

    // An array inside a boxed struct: a pointer to the field that shares the box.
    gp::ptr<Holder> h = gp::new_<Holder>();
    ints hs = gp::sub(gp::ptr<iarr<3>>(h, &h->arr), 1);
    hs[0] = 7;
    CHECK(h->arr[1] == 7);
    gp::weak_ptr<Holder> wh = gp::weak_make(h);
    h = nullptr;
    CHECK(gp::weak_value(wh) != nullptr);
    hs = nullptr;
    CHECK(gp::weak_value(wh) == nullptr);

    // An array inside a slice element shares the slice's backing array.
    gp::slice<Holder> hsl = gp::make_slice<Holder>(2);
    ints es = gp::sub(gp::ptr<iarr<3>>(hsl.gp_owner(), &hsl[1].arr), 0, 2);
    es[1] = 9;
    CHECK(hsl[1].arr[1] == 9);

    // A package-level array shares the static owner.
    ints gs = gp::sub(gp::static_ptr(global_arr), 1, 3);
    gs[0] = 22;
    CHECK(global_arr[1] == 22 && gs.gp_owner().use_count() > 0);
    gp::ptr<int64_t> e0 = gp::ptr<int64_t>(gs.gp_owner(), &gs[1]); // &s[i] has an owner

    // &arr[i] of a boxed array shares the box.
    gp::ptr<iarr<2>> b2 = gp::new_<iarr<2>>();
    gp::ptr<int64_t> pe = gp::ptr<int64_t>(b2, &(*b2)[1]);
    b2 = nullptr;
    *pe = 5;
    CHECK(*pe == 5);

    // [0]T: an empty, non-nil slice.
    gp::ptr<iarr<0>> z = gp::new_<iarr<0>>();
    ints zs = gp::sub(z, 0);
    CHECK(zs != nullptr && gp::len(zs) == 0 && gp::cap(zs) == 0);
    CHECK(gp::sub(z, 0, 0, 0) != nullptr);

    // Go's bounds checks for arrays say "with length N".
    gp::ptr<iarr<3>> b = gp::new_<iarr<3>>();
    std::string p = "runtime error: slice bounds out of range ";
    CHECK_EQ(panic_text([&] { gp::sub(b, 0, 4); }), p + "[:4] with length 3");
    CHECK_EQ(panic_text([&] { gp::sub(b, 0, -1); }), p + "[:-1]");
    CHECK_EQ(panic_text([&] { gp::sub(b, 0, UINT64_MAX); }),
             p + "[:18446744073709551615] with length 3");
    CHECK_EQ(panic_text([&] { gp::sub(b, 2, 1); }), p + "[2:1]");
    CHECK_EQ(panic_text([&] { gp::sub(b, 4); }), p + "[4:3]");
    CHECK_EQ(panic_text([&] { gp::sub(b, -1); }), p + "[-1:]");
    CHECK_EQ(panic_text([&] { gp::sub(b, 0, 1, 4); }), p + "[::4] with length 3");
    CHECK_EQ(panic_text([&] { gp::sub(b, 0, 3, 2); }), p + "[:3:2]");
    CHECK_EQ(panic_text([&] { gp::sub(b, 2, 1, 3); }), p + "[2:1:]");
    CHECK_EQ(panic_text([&] { gp::sub(z, 0, 1); }), p + "[:1] with length 0");
    CHECK_EQ(panic_text([&] { gp::sub(z, 1); }), p + "[1:0]");
    // A nil pointer to an array panics with the nil dereference first, as in Go.
    std::string nil = "runtime error: invalid memory address or nil pointer dereference";
    CHECK_EQ(panic_text([] { gp::sub(gp::ptr<iarr<3>>(), 0, 5); }), nil);
    CHECK_EQ(panic_text([] { gp::sub(gp::ptr<iarr<3>>(), 0); }), nil);
    CHECK_EQ(panic_text([] { gp::sub(gp::ptr<iarr<3>>(), 0, 0, 0); }), nil);
    CHECK_EQ(panic_text([] { gp::sub(gp::ptr<iarr<0>>(), 0); }), nil);
    gp::detail::clear_signal(); // the nil panics above set the goroutine's signal line

    // len and cap of a pointer to an array don't dereference it.
    gp::ptr<iarr<3>> np;
    CHECK(gp::len(np) == 3 && gp::cap(np) == 3);
    (void)e0;
}

// --- range loops and elem_ptr ---

// A large array (over 128 KiB): range_array_copy copies it to the heap.
using big = gp::array<int64_t, 20000>;
static_assert(gp::detail::large_array<int64_t, 20000> && !gp::detail::large_array<int64_t, 16384>);
static_assert(sizeof(gp::array_copy_range<int64_t, 20000>) < 64);
static_assert(sizeof(gp::array_copy_range<int64_t, 3>) == 32);

void test_range() {
    // range_array reads the array itself: writes in the body are seen (the emitter uses it
    // only where the body can't write).
    iarr<3> a{1, 2, 3};
    std::vector<int64_t> got;
    for (auto it = gp::range_array(a); gp::range_next(it);) {
        if (it.gp_i == 0) {
            a[2] = 30;
        }
        got.push_back(it.gp_i * 100 + it.gp_v());
    }
    CHECK(got == std::vector<int64_t>({1, 102, 230}));

    // range_array_copy ranges over a copy: writes in the body are not seen.
    got.clear();
    for (auto it = gp::range_array_copy(a); gp::range_next(it);) {
        a[it.gp_i] = -1;
        got.push_back(it.gp_v());
    }
    CHECK(got == std::vector<int64_t>({1, 2, 30}) && same(a, {-1, -1, -1}));
    // Of a temporary (f() in Go): the copy outlives it.
    got.clear();
    for (auto it = gp::range_array_copy(iarr<2>{7, 8}); gp::range_next(it);) {
        got.push_back(it.gp_v());
    }
    CHECK(got == std::vector<int64_t>({7, 8}));
    // An rvalue is moved into the range (range_array_copy(array&&)), also a large one.
    gp::array<gp::string, 2> strs{gp::concat("hel"_gs, "lo"_gs), "x"_gs};
    gp::string joined;
    for (auto it = gp::range_array_copy(std::move(strs)); gp::range_next(it);) {
        joined = joined + it.gp_v();
    }
    CHECK(joined == "hellox"_gs);
    int64_t zeros = 0;
    for (auto it = gp::range_array_copy(big{}); gp::range_next(it);) {
        zeros += it.gp_v() == 0 ? 1 : 0;
    }
    CHECK(zeros == 20000);
    // A large array is copied to the heap.
    gp::ptr<big> b = gp::new_<big>();
    (*b)[19999] = 5;
    int64_t sum = 0, n = 0;
    for (auto it = gp::range_array_copy(*b); gp::range_next(it);) {
        (*b)[19999] = 0;
        sum += it.gp_v();
        n++;
    }
    CHECK(sum == 5 && n == 20000);

    // range_array of a pointer reads through it: writes are seen, and a nil pointer panics at
    // the first element (none for [0]T).
    gp::ptr<iarr<3>> p = gp::new_<iarr<3>>(iarr<3>{1, 2, 3});
    got.clear();
    for (auto it = gp::range_array(p); gp::range_next(it);) {
        if (it.gp_i == 0) {
            (*p)[1] = 20;
            p = nullptr; // the loop holds its own copy of the pointer
        }
        got.push_back(it.gp_v());
    }
    CHECK(got == std::vector<int64_t>({1, 20, 3}));
    std::string nil = "runtime error: invalid memory address or nil pointer dereference";
    CHECK_EQ(panic_text([] {
                 for (auto it = gp::range_array(gp::ptr<iarr<3>>()); gp::range_next(it);) {
                     (void)it.gp_v();
                 }
             }),
             nil);
    gp::detail::clear_signal();
    n = 0;
    for (auto it = gp::range_array(gp::ptr<iarr<0>>()); gp::range_next(it);) {
        n++;
    }
    for (auto it = gp::range_array_copy(iarr<0>{}); gp::range_next(it);) {
        n++;
    }
    CHECK(n == 0);

    // new_array, new_array_keyed, new_to_array: built in the new block, the rest zero.
    gp::ptr<big> nb = gp::new_array<int64_t, 20000>({1, 2, 3});
    CHECK((*nb)[0] == 1 && (*nb)[2] == 3 && (*nb)[3] == 0 && (*nb)[19999] == 0);
    gp::ptr<big> nk = gp::new_array_keyed<int64_t, 20000>({{19999, 9}, {4, 5}});
    CHECK((*nk)[19999] == 9 && (*nk)[4] == 5 && (*nk)[0] == 0);
    gp::ptr<gp::array<gp::string, 3>> ns = gp::new_array<gp::string, 3>({"a"_gs});
    CHECK((*ns)[0] == "a"_gs && (*ns)[2] == ""_gs);
    ints src = gp::make_slice<int64_t>(20001);
    src[19999] = 7;
    gp::ptr<big> nt = gp::new_to_array<20000>(src);
    src[19999] = 70;
    CHECK((*nt)[19999] == 7);
    CHECK_EQ(panic_text([&] { gp::new_to_array<20000>(gp::sub(src, 2)); }),
             std::string("runtime error: cannot convert slice with length 19999 to array or "
                         "pointer to array with length 20000"));
    CHECK(gp::new_to_array<0>(ints()) != nullptr);

    // elem_ptr: &(*p)[i] sharing p's block, with the nil check before the index check.
    gp::ptr<iarr<3>> q = gp::new_<iarr<3>>();
    gp::ptr<int64_t> e = gp::elem_ptr(q, 2);
    gp::weak_ptr<iarr<3>> wq = gp::weak_make(q);
    q = nullptr;
    *e = 9;
    CHECK(*e == 9 && gp::weak_value(wq) != nullptr);
    e = nullptr;
    CHECK(gp::weak_value(wq) == nullptr);
    gp::ptr<gp::array<iarr<2>, 2>> g = gp::new_<gp::array<iarr<2>, 2>>();
    gp::ptr<int64_t> ge = gp::elem_ptr(gp::elem_ptr(g, 1), 0); // &g[1][0]
    *ge = 4;
    CHECK((*g)[1][0] == 4);
    CHECK_EQ(panic_text([] { gp::elem_ptr(gp::ptr<iarr<3>>(), 5); }), nil);
    gp::detail::clear_signal();
    CHECK_EQ(panic_text([&] { gp::elem_ptr(g, int8_t(-1)); }),
             std::string("runtime error: index out of range [-1]"));
}

// --- argument-dependent lookup ---

void test_adl() {
    using gpkg_adl::P;
    gp::array<P, 3> a{P{1}, P{2}};
    a[2] = P{3};
    CHECK(a[1].v == 2 && a == a);
    CHECK_EQ(panic_text([&] { a[3]; }),
             std::string("runtime error: index out of range [3] with length 3"));
    gp::slice<P> s = gp::slice_of<P>({P{4}, P{5}, P{6}});
    gp::array<P, 2> c = gp::to_array<2>(s);
    gp::ptr<gp::array<P, 2>> cp = gp::to_array_ptr<2>(s);
    gp::ptr<gp::array<P, 3>> box = gp::new_<gp::array<P, 3>>(a);
    gp::slice<P> bs = gp::sub(box, INT64_C(1));
    gp::slice<P> bs3 = gp::sub(box, 0, 1, 2);
    CHECK(c[1].v == 5 && (*cp)[0].v == 4 && bs[0].v == 2 && gp::cap(bs3) == 2);
    CHECK(gp::len(a) == 3 && gp::len(box) == 3);
    gp::array<P, 2> k = gp::array_keyed<P, 2>({{1, P{8}}});
    CHECK(k[1].v == 8);
    CHECK_EQ(gpkg_adl::hijacked, INT64_C(0));
}

void run_tests() {
    test_zero_and_literals();
    test_values();
    test_index();
    test_equal();
    test_lifetimes();
    test_chains();
    test_convert();
    test_views_alias();
    test_sub();
    test_range();
    test_adl();
    if (failures > 0) {
        std::string msg =
            "FAIL: " + std::to_string(failures) + " of " + std::to_string(checks) + " checks\n";
        gp::os::write(gp::os::stderr_fd, msg.data(), msg.size());
        gp::os::exit(1);
    }
    gp::fmtshim::Printf("ok: %d checks\n"_gs, int64_t(checks));
}

void no_init() {}

} // namespace

int main(int argc, char** argv) {
    return gp::run_main(argc, argv, &no_init, &run_tests);
}
