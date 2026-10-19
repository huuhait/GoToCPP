// The unit tests of gp::slice (gocpp/slice.hpp) and of the slice bounds checks
// (gocpp/bounds.hpp). runtime/cxx_test.go compiles this file with the runtime's .cpp files,
// with g++ and clang++, at -O0, at -O2 and with ASan + UBSan (LeakSanitizer included), and
// runs it like test/unit.cpp. It prints "ok: N checks" and exits 0, or prints each failed check
// and exits 1. The Go behavior it checks is in runtime/testdata/diff/main.go (sliceCase) too,
// where real Go runs it.
#include <gocpp/rt.hpp>
#include <gocpp/os.hpp>

#include <concepts>
#include <cstdint>
#include <memory>
#include <string>
#include <type_traits>
#include <utility>
#include <vector>

// A user's package (a namespace like generated code's gpkg_main) with Go functions whose names
// are also the names of the runtime's helpers. The runtime's calls are qualified, so
// argument-dependent lookup for a slice of a user type never picks these (test_adl).
namespace gpkg_adl {

struct P {
    int64_t v = 0;
};

int64_t hijacked = 0;

template <typename T>
gp::slice<T> grow(const gp::slice<T>& s, int64_t) {
    hijacked++;
    return s;
}
template <typename T>
void copy_elems(T*, const T*, int64_t) {
    hijacked++;
}
template <typename T>
gp::slice<T> append_n(gp::slice<T> s, const T*, int64_t) {
    hijacked++;
    return s;
}
gp::slice<P> sub(gp::slice<P> s, int64_t, int64_t) {
    hijacked++;
    return s;
}
template <typename T>
std::shared_ptr<T[]> new_array(int64_t, int64_t) {
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
using bytes = gp::slice<uint8_t>;
using strs = gp::slice<gp::string>;

// elems: the elements of s, for comparisons.
template <typename T>
std::vector<T> elems(const gp::slice<T>& s) {
    std::vector<T> v;
    for (int64_t i = 0; i < gp::len(s); i++) {
        v.push_back(s[i]);
    }
    return v;
}

bool same(const ints& s, std::vector<int64_t> want) {
    return elems(s) == want;
}

// --- layout, zero values, the type's rules ---

// A slice accepts an incomplete element type: Node is incomplete where its member is declared.
struct Node {
    gp::slice<Node> kids;
    int64_t v = 0;
};

// A zero-value package-level slice needs no dynamic initialization.
constinit ints global_nil;
int64_t global_arr[3] = {1, 2, 3};

static_assert(sizeof(gp::slice<int64_t>) == 40);
static_assert(sizeof(gp::slice<Node>) == 40);
static_assert(sizeof(gp::slice<gp::string>) == 40);
static_assert(!std::equality_comparable<ints>, "Go compares slices only with nil");
static_assert(requires(ints s) { s == nullptr; });
static_assert(requires(ints s) { s != nullptr; });
static_assert(std::is_nothrow_move_constructible_v<ints>);
static_assert(std::is_same_v<decltype(std::declval<const ints&>()[0]), int64_t&>,
              "s[i] is a writable element, also for a const slice");
static_assert(!std::is_convertible_v<ints, bool>);

void test_nil_and_empty() {
    ints n;
    CHECK(n == nullptr);
    CHECK(global_nil == nullptr);
    CHECK_EQ(gp::len(n), INT64_C(0));
    CHECK_EQ(gp::cap(n), INT64_C(0));
    CHECK(gp::sub(n, 0, 0) == nullptr);
    CHECK(gp::sub(n, 0) == nullptr);
    CHECK(gp::sub(n, 0, 0, 0) == nullptr);
    CHECK(gp::append(n) == nullptr);
    CHECK(gp::append_slice(n, ints()) == nullptr);
    CHECK(gp::append_slice(bytes(), ""_gs) == nullptr);
    CHECK_EQ(gp::copy(n, gp::slice_of<int64_t>({1})), INT64_C(0));
    gp::clear(n);

    // []T{} and make([]T, 0) are empty but not nil, and allocate nothing.
    ints e = gp::slice_of<int64_t>({});
    ints m = gp::make_slice<int64_t>(0);
    ints m2 = gp::make_slice<int64_t>(0, 0);
    CHECK(e != nullptr && m != nullptr && m2 != nullptr);
    CHECK(e.gp_owner() == nullptr);
    CHECK(gp::len(e) == 0 && gp::cap(e) == 0);
    CHECK(gp::sub(e, 0, 0) != nullptr); // a slice of an empty slice is not nil
    CHECK(gp::append(e) != nullptr);
    CHECK(gp::append_slice(m, ints()) != nullptr);

    // nil assignment and nil arguments
    ints s = gp::slice_of<int64_t>({1, 2});
    s = nullptr;
    CHECK(s == nullptr);
    auto f = [](const ints& x) { return x == nullptr; };
    CHECK(f(nullptr));

    // A move leaves nil behind.
    ints a = gp::slice_of<int64_t>({1, 2});
    ints b = std::move(a);
    CHECK(a == nullptr && gp::len(a) == 0 && gp::cap(a) == 0); // NOLINT(bugprone-use-after-move)
    CHECK(same(b, {1, 2}));
    ints c;
    c = std::move(b);
    CHECK(b == nullptr && same(c, {1, 2})); // NOLINT(bugprone-use-after-move)
    ints& cr = c;
    c = std::move(cr); // self-move: unchanged
    CHECK(same(c, {1, 2}));
    c = cr; // self-copy
    CHECK(same(c, {1, 2}));
}

// Plain is trivial: make and growth zero it with memset. Init has default member initializers
// (like generated structs); Holder holds a string, so it is constructed and assigned.
struct Plain {
    int64_t a;
    double b;
    bool c;
};
struct Init {
    int64_t a = 0;
    int64_t b = 0;
};
struct Holder {
    gp::string s;
    int64_t n;
};

void test_make() {
    ints m = gp::make_slice<int64_t>(3, 10);
    CHECK_EQ(gp::len(m), INT64_C(3));
    CHECK_EQ(gp::cap(m), INT64_C(10));
    CHECK(same(gp::sub(m, 0, 10), std::vector<int64_t>(10, 0)));
    CHECK_EQ(gp::cap(gp::make_slice<int64_t>(5)), INT64_C(5));
    CHECK_EQ(gp::cap(gp::make_slice<int64_t>(uint8_t(4), uint16_t(7))), INT64_C(7));

    gp::slice<Plain> p = gp::make_slice<Plain>(2, 4);
    bool zero = true;
    gp::slice<Plain> p4 = gp::sub(p, 0, 4);
    for (int64_t i = 0; i < 4; i++) {
        const Plain& x = p4[i];
        zero = zero && x.a == 0 && x.b == 0.0 && !x.c;
    }
    CHECK(zero);
    gp::slice<Holder> h = gp::make_slice<Holder>(3);
    CHECK(h[2].s == ""_gs && h[2].n == 0);
    strs s = gp::make_slice<gp::string>(2);
    CHECK(s[0] == ""_gs && s[1] == ""_gs);
    gp::slice<gp::ptr<int64_t>> ps = gp::make_slice<gp::ptr<int64_t>>(2);
    CHECK(ps[1] == nullptr);
    gp::slice<ints> nested = gp::make_slice<ints>(2);
    CHECK(nested[0] == nullptr);

    // Go's makeslice panics.
    CHECK_EQ(panic_text([] { gp::make_slice<int64_t>(-1); }),
             std::string("runtime error: makeslice: len out of range"));
    CHECK_EQ(panic_text([] { gp::make_slice<int64_t>(INT64_C(1) << 60); }),
             std::string("runtime error: makeslice: len out of range"));
    CHECK_EQ(panic_text([] { gp::make_slice<int64_t>(UINT64_MAX); }),
             std::string("runtime error: makeslice: len out of range"));
    CHECK_EQ(panic_text([] { gp::make_slice<int64_t>(2, 1); }),
             std::string("runtime error: makeslice: cap out of range"));
    CHECK_EQ(panic_text([] { gp::make_slice<int64_t>(1, -1); }),
             std::string("runtime error: makeslice: cap out of range"));
    CHECK_EQ(panic_text([] { gp::make_slice<int64_t>(1, INT64_C(1) << 60); }),
             std::string("runtime error: makeslice: cap out of range"));
    CHECK_EQ(panic_text([] { gp::make_slice<int64_t>(-1, 5); }),
             std::string("runtime error: makeslice: len out of range"));
    // The limit is Go's maxAlloc, 1 << 48 bytes.
    CHECK_EQ(panic_text([] { gp::make_slice<uint8_t>(0, (INT64_C(1) << 48) + 1); }),
             std::string("runtime error: makeslice: cap out of range"));
}

void test_index() {
    ints s = gp::slice_of<int64_t>({10, 20, 30});
    CHECK_EQ(s[0], INT64_C(10));
    CHECK_EQ(s[uint8_t(2)], INT64_C(30));
    s[int8_t(1)] = 21;
    CHECK_EQ(s[uint64_t(1)], INT64_C(21));
    CHECK_EQ(panic_text([&] { s[3]; }),
             std::string("runtime error: index out of range [3] with length 3"));
    CHECK_EQ(panic_text([&] { s[-1]; }), std::string("runtime error: index out of range [-1]"));
    CHECK_EQ(panic_text([&] { s[UINT64_MAX]; }),
             std::string("runtime error: index out of range [18446744073709551615] with length 3"));
    CHECK_EQ(panic_text([] { ints().operator[](0); }),
             std::string("runtime error: index out of range [0] with length 0"));
    // The index is checked against len, not cap.
    ints t = gp::sub(gp::make_slice<int64_t>(2, 8), 0, 1);
    CHECK_EQ(panic_text([&] { t[1]; }),
             std::string("runtime error: index out of range [1] with length 1"));
}

void test_sub() {
    ints s = gp::make_slice<int64_t>(3, 5);
    for (int64_t i = 0; i < 3; i++) {
        s[i] = i + 1;
    }
    ints t = gp::sub(s, 1, 4);
    CHECK(gp::len(t) == 3 && gp::cap(t) == 4 && same(t, {2, 3, 0}));
    CHECK(gp::len(gp::sub(s, 2)) == 1 && gp::cap(gp::sub(s, 2)) == 3);
    ints u = gp::sub(s, 1, 2, 3);
    CHECK(gp::len(u) == 1 && gp::cap(u) == 2);
    CHECK_EQ(gp::cap(gp::sub(s, 5, 5)), INT64_C(0));
    CHECK(gp::sub(s, 5, 5).gp_data() == s.gp_data()); // like Go: cap 0 keeps the pointer
    CHECK(gp::sub(s, 0, 0).gp_data() == s.gp_data());
    CHECK(gp::sub(s, 2, 2).gp_data() == s.gp_data() + 2);
    CHECK_EQ(gp::cap(gp::sub(gp::sub(s, 0, 2, 2), 0, 2)), INT64_C(2));
    // A slice can grow back up to its capacity.
    CHECK(same(gp::sub(gp::sub(s, 0, 1), 0, 3), {1, 2, 3}));
    // Index types mix.
    CHECK_EQ(gp::len(gp::sub(s, uint8_t(1), int16_t(3), uint64_t(5))), INT64_C(2));

    auto msg = [](auto f) { return panic_text(f); };
    std::string p = "runtime error: slice bounds out of range ";
    CHECK_EQ(msg([&] { gp::sub(s, 0, 6); }), p + "[:6] with capacity 5");
    CHECK_EQ(msg([&] { gp::sub(s, 0, -1); }), p + "[:-1]");
    CHECK_EQ(msg([&] { gp::sub(s, 0, UINT64_MAX); }),
             p + "[:18446744073709551615] with capacity 5");
    CHECK_EQ(msg([&] { gp::sub(s, 3, 2); }), p + "[3:2]");
    CHECK_EQ(msg([&] { gp::sub(s, -2, 2); }), p + "[-2:]");
    CHECK_EQ(msg([&] { gp::sub(s, uint64_t(1) << 63, 2); }), p + "[9223372036854775808:2]");
    CHECK_EQ(msg([&] { gp::sub(s, 4); }), p + "[4:3]");
    CHECK_EQ(msg([&] { gp::sub(s, 6, 6); }), p + "[:6] with capacity 5"); // hi is checked first
    CHECK_EQ(msg([&] { gp::sub(ints(), 0, 1); }), p + "[:1] with capacity 0");
    CHECK_EQ(msg([&] { gp::sub(s, 0, 1, 6); }), p + "[::6] with capacity 5");
    CHECK_EQ(msg([&] { gp::sub(s, 0, 1, -1); }), p + "[::-1]");
    CHECK_EQ(msg([&] { gp::sub(s, 0, 4, 2); }), p + "[:4:2]");
    CHECK_EQ(msg([&] { gp::sub(s, 0, -1, 2); }), p + "[:-1:]");
    CHECK_EQ(msg([&] { gp::sub(s, 3, 2, 4); }), p + "[3:2:]");
    CHECK_EQ(msg([&] { gp::sub(s, -1, 2, 4); }), p + "[-1::]");
    CHECK_EQ(msg([&] { gp::sub(s, 9, 8, 7); }), p + "[::7] with capacity 5"); // max first
    CHECK_EQ(msg([&] { gp::sub(s, int8_t(-1), 2); }), p + "[-1:]");

    // The checks for strings and arrays say "length" (bounds.hpp; used by steps 3.4, 3.6).
    CHECK_EQ(msg([] { gp::check_slice(0, 4, 3, gp::bounds::slice_alen); }),
             p + "[:4] with length 3");
    CHECK_EQ(msg([] { gp::check_slice3(0, 1, 4, 3, gp::bounds::slice3_alen); }),
             p + "[::4] with length 3");
    CHECK_EQ(msg([] { gp::check_slice(2, 1, 3, gp::bounds::slice_alen); }), p + "[2:1]");
    CHECK_EQ(msg([] { gp::check_slice(0, 3, 3, gp::bounds::slice_alen); }), "<no panic>");

    // [N]T(s): N <= len(s).
    CHECK_EQ(msg([] { gp::check_convert(3, 2); }),
             std::string("runtime error: cannot convert slice with length 2 to array or pointer "
                         "to array with length 3"));
    CHECK_EQ(msg([] { gp::check_convert(2, 2); }), "<no panic>");
    CHECK_EQ(msg([] { gp::check_convert(0, 0); }), "<no panic>");
}

// --- aliasing: slices of one array ---

void test_aliasing() {
    ints a = gp::slice_of<int64_t>({1, 2, 3, 4, 5});
    ints b = gp::sub(a, 1, 3);
    b[0] = 20;
    CHECK_EQ(a[1], INT64_C(20));
    a[2] = 30;
    CHECK_EQ(b[1], INT64_C(30));

    // append within capacity writes into the shared array: a sees it.
    ints b2 = gp::append(b, INT64_C(40));
    CHECK(same(a, {1, 20, 30, 40, 5}));
    CHECK(b2.gp_data() == b.gp_data());
    // ... and a second append from the same b overwrites the same element.
    ints b3 = gp::append(b, INT64_C(41));
    CHECK_EQ(b2[2], INT64_C(41));
    CHECK(same(b3, {20, 30, 41}));

    // append beyond capacity copies: the new slice no longer shares.
    ints c = gp::append(gp::sub(a, 1, 3, 3), INT64_C(99));
    c[0] = -1;
    CHECK(same(a, {1, 20, 30, 41, 5}));
    CHECK(same(c, {-1, 30, 99}));

    // append_slice within capacity writes into the shared array too.
    ints d = gp::append_slice(gp::sub(a, 0, 1), gp::slice_of<int64_t>({7, 8}));
    CHECK(same(a, {1, 7, 8, 41, 5}));
    CHECK(d.gp_data() == a.gp_data());
}

// --- append ---

volatile int64_t fake_len = INT64_MAX;

void test_append() {
    ints x;
    x = gp::append(x, INT64_C(1));
    CHECK(same(x, {1}));
    x = gp::append(x, 2, 3, 4); // int arguments convert to int64_t
    CHECK(same(x, {1, 2, 3, 4}));

    // Self-append, with and without room.
    ints y = x;
    y = gp::append_slice(y, y);
    CHECK(same(y, {1, 2, 3, 4, 1, 2, 3, 4}));
    ints r = gp::make_slice<int64_t>(2, 8);
    r[0] = 7;
    r[1] = 8;
    r = gp::append_slice(r, r);
    r = gp::append_slice(r, r);
    CHECK(same(r, {7, 8, 7, 8, 7, 8, 7, 8}));
    CHECK_EQ(gp::cap(r), INT64_C(8));

    // Removing an element in place, and overlapping appends inside one array.
    ints z = gp::slice_of<int64_t>({0, 1, 2, 3, 4});
    z = gp::append_slice(gp::sub(z, 0, 1), gp::sub(z, 2));
    CHECK(same(z, {0, 2, 3, 4}));
    ints w = gp::make_slice<int64_t>(5, 10);
    for (int64_t i = 0; i < 5; i++) {
        w[i] = i;
    }
    ints w2 = gp::append_slice(gp::sub(w, 0, 1), gp::sub(w, 0, 4)); // dst after src
    CHECK(same(w2, {0, 0, 1, 2, 3}));
    ints w3 = gp::append_slice(gp::sub(w, 0, 4), gp::sub(w, 0, 6)); // grows: reads the old array
    CHECK(same(w3, {0, 0, 1, 2, 0, 0, 1, 2, 3, 0}));

    // The arguments are read before anything is stored (Go evaluates them first).
    ints v = gp::slice_of<int64_t>({1, 2, 3, 4});
    v = gp::append(gp::sub(v, 0, 1), v[2], v[1]);
    CHECK(same(v, {1, 3, 2}));
    ints v2 = gp::slice_of<int64_t>({1, 2, 3});
    v2 = gp::append(gp::sub(v2, 0, 1, 1), v2[0], v2[2]); // grows: reads the old array
    CHECK(same(v2, {1, 1, 3}));

    // A string's bytes appended to a []byte.
    bytes bs = gp::slice_of<uint8_t>({'g'});
    bs = gp::append_slice(bs, "o!"_gs);
    CHECK(elems(bs) == std::vector<uint8_t>({'g', 'o', '!'}));
    bytes nb;
    nb = gp::append_slice(nb, "abc"_gs);
    CHECK(gp::len(nb) == 3 && nb[2] == 'c');

    // The capacity follows Go's growth formula and size classes (test_go_caps has every kind).
    ints g;
    g = gp::append(g, 1, 2, 3);
    CHECK_EQ(gp::cap(g), INT64_C(3));
    g = gp::append(g, 1, 2, 3, 4, 5, 6, 7, 8);
    CHECK_EQ(gp::cap(g), INT64_C(12));
    bytes gb = gp::make_slice<uint8_t>(0);
    gb = gp::append_slice(gb, "hello, world!"_gs);
    CHECK_EQ(gp::cap(gb), INT64_C(16));

    // The new array's elements after len are zero values (also for a trivial type).
    ints t = gp::append(ints(), INT64_C(5));
    t = gp::append(t, INT64_C(6)); // cap 2
    t = gp::append(t, INT64_C(7)); // cap 4
    CHECK(same(gp::sub(t, 0, gp::cap(t)), {5, 6, 7, 0}));
    strs ts = gp::append(strs(), "a"_gs, "b"_gs, "c"_gs);
    ts = gp::append(ts, "d"_gs);
    CHECK(gp::cap(ts) > 4 && gp::sub(ts, 0, gp::cap(ts))[4] == ""_gs);

    // Go's growslice panic, when the length would overflow or the array be too big. The
    // slices are made up and never read; their lengths are volatile, so that g++ -O2 doesn't
    // warn about the copy that the panic stops (-Warray-bounds).
    CHECK_EQ(panic_text([] {
                 bytes huge(gp::static_owner(), gp::detail::empty_data<uint8_t>(), fake_len,
                            fake_len);
                 gp::append(huge, uint8_t(1));
             }),
             std::string("runtime error: growslice: len out of range"));
    CHECK_EQ(panic_text([] {
                 int64_t n = fake_len >> 15; // (1 << 48) - 1
                 bytes big(gp::static_owner(), gp::detail::empty_data<uint8_t>(), n, n);
                 gp::append(big, uint8_t(1));
             }),
             std::string("runtime error: growslice: len out of range"));
}

// The capacities of Go 1.25 for appends one at a time (cap changes of a slice that escapes to
// the heap; printed by real Go on amd64): int64 elements, bytes and 24-byte elements.
const std::vector<int64_t> go_caps_int = {
    1,      2,      4,      8,      16,     32,     64,     128,    256,    512,
    848,    1280,   1792,   2560,   3408,   5120,   7168,   9216,   12288,  16384,
    21504,  27648,  34816,  44032,  55296,  69632,  88064,  110592, 139264, 175104,
    219136, 274432, 344064, 431104, 539648, 674816, 843776, 1055744};
const std::vector<int64_t> go_caps_byte = {8,     16,    32,    64,    128,   256,   512,   896,
                                           1408,  2048,  3072,  4096,  5376,  6912,  9472,  12288,
                                           16384, 21760, 28672, 40960, 57344, 73728, 98304, 131072};
const std::vector<int64_t> go_caps_24 = {1,     2,     4,     8,     16,    32,    64,    128,
                                         256,   512,   853,   1365,  2048,  3072,  4096,  5461,
                                         7168,  9216,  11946, 15360, 19456, 24576, 31061, 39253,
                                         49493, 62122, 78165, 97962, 122880};

struct Three {
    int64_t a, b, c;
};

template <typename T>
std::vector<int64_t> grow_caps(int64_t n, int64_t* reallocs, bool* ok) {
    gp::slice<T> s;
    std::vector<int64_t> caps;
    *reallocs = 0;
    *ok = true;
    for (int64_t i = 0; i < n; i++) {
        T* before = s.gp_data();
        s = gp::append(s, T{});
        if (s.gp_data() != before) {
            ++*reallocs;
            caps.push_back(gp::cap(s));
        }
        *ok = *ok && gp::cap(s) >= gp::len(s) && gp::len(s) == i + 1;
    }
    return caps;
}

void test_growth() {
    int64_t reallocs = 0;
    bool ok = false;
    CHECK(grow_caps<int64_t>(1'000'000, &reallocs, &ok) == go_caps_int);
    CHECK(ok);
    CHECK(reallocs <= 40); // amortized O(1): about 1.25x per reallocation
    CHECK(grow_caps<uint8_t>(100'000, &reallocs, &ok) == go_caps_byte);
    CHECK(ok);
    CHECK(grow_caps<Three>(100'000, &reallocs, &ok) == go_caps_24);
    CHECK(ok);

    // The values survive the reallocations.
    ints s;
    for (int64_t i = 0; i < 1'000'000; i++) {
        s = gp::append(std::move(s), i);
    }
    int64_t sum = 0;
    for (int64_t i = 0; i < gp::len(s); i++) {
        sum += s[i];
    }
    CHECK_EQ(sum, INT64_C(499999500000));

    // Strings: elements copied into a new array keep their bytes.
    strs ss;
    for (int64_t i = 0; i < 10'000; i++) {
        ss = gp::append(ss, gp::string::gp_copy(std::to_string(i)));
    }
    CHECK(ss[9999] == "9999"_gs && ss[0] == "0"_gs);
}

// --- Go's capacities for every kind of element (gocpp/golayout.hpp) ---

struct Empty { // struct{}, as the emitter writes it
    bool operator==(const Empty&) const = default;
    static constexpr auto gp_fields = gp::fields();
};
struct GoPair { // struct{ a int; p *int }
    int64_t a = 0;
    gp::ptr<int64_t> p;
    static constexpr auto gp_fields = gp::fields(gp::field(&GoPair::a, "a", false, false),
                                                 gp::field(&GoPair::p, "p", false, false));
};
struct GoTail { // struct{ a int; z struct{} }: gc pads a zero-size last field
    int64_t a = 0;
    Empty z;
    static constexpr auto gp_fields = gp::fields(gp::field(&GoTail::a, "a", false, false),
                                                 gp::field(&GoTail::z, "z", false, false));
};
struct GoTail4 { // struct{ y int32; z [0]*int }: z is aligned as a pointer
    int32_t y = 0;
    gp::array<gp::ptr<int64_t>, 0> z;
    static constexpr auto gp_fields = gp::fields(gp::field(&GoTail4::y, "y", false, false),
                                                 gp::field(&GoTail4::z, "z", false, false));
};
struct GoZero { // struct{ z [0]int }: zero size in Go, but not an empty class in C++
    gp::array<int64_t, 0> z;
    static constexpr auto gp_fields = gp::fields(gp::field(&GoZero::z, "z", false, false));
};
using GoZero3 = gp::array<GoZero, 3>;
struct GoMarker { // struct{ _ [0]func() }
    gp::array<gp::func<void()>, 0> gp_blank;
    static constexpr auto gp_fields = gp::fields(gp::field(&GoMarker::gp_blank, "_", false, false));
};
struct GoNamed : gp::newtype<GoNamed, gp::slice<int64_t>> {
    using gp::newtype<GoNamed, gp::slice<int64_t>>::newtype;
};
struct GoCelsius : gp::newtype<GoCelsius, double> {
    using gp::newtype<GoCelsius, double>::newtype;
};

namespace lay = gp::detail;
template <typename T>
constexpr lay::go_layout_t L = lay::go_layout<T>();

static_assert(L<int64_t>.size == 8 && !L<int64_t>.ptrs);
static_assert(L<int8_t>.size == 1 && L<bool>.size == 1 && L<float>.size == 4);
static_assert(L<gp::complex128>.size == 16 && !L<gp::complex128>.ptrs);
static_assert(L<gp::string>.size == 16 && L<gp::string>.ptrs, "Go's string is 16 bytes");
static_assert(L<gp::any>.size == 16 && L<gp::any>.ptrs && L<gp::error>.size == 16);
static_assert(L<gp::ptr<int64_t>>.size == 8 && L<gp::ptr<int64_t>>.ptrs);
static_assert(L<gp::ptr<void>>.size == 8 && L<gp::weak_ptr<int64_t>>.size == 8);
static_assert(L<gp::slice<int64_t>>.size == 24 && L<gp::slice<int64_t>>.ptrs);
static_assert(L<gp::map<int64_t, int64_t>>.size == 8 && L<gp::chan<int64_t>>.size == 8);
static_assert(L<gp::func<void()>>.size == 8 && L<gp::func<void()>>.ptrs);
static_assert(L<gp::array<gp::string, 4>>.size == 64 && L<gp::array<gp::string, 4>>.ptrs);
static_assert(L<gp::array<int16_t, 3>>.size == 6 && !L<gp::array<int16_t, 3>>.ptrs);
static_assert(L<gp::array<gp::string, 0>>.size == 0 && !L<gp::array<gp::string, 0>>.ptrs);
static_assert(L<GoPair>.size == 16 && L<GoPair>.ptrs);
static_assert(L<GoTail>.size == 16 && !L<GoTail>.ptrs);
static_assert(L<GoTail4>.size == 16 && L<GoTail4>.align == 8 && !L<GoTail4>.ptrs);
static_assert(L<GoZero>.size == 0 && L<GoZero3>.size == 0 && L<GoMarker>.size == 0);
static_assert(!std::is_empty_v<GoZero> && !std::is_empty_v<GoZero3>);
static_assert(L<GoNamed>.size == 24 && L<GoNamed>.ptrs);
static_assert(L<GoCelsius>.size == 8 && !L<GoCelsius>.ptrs);
static_assert(L<Empty>.size == 0 && !L<Empty>.ptrs);
struct Foreign {};
static_assert(L<Foreign>.size == 1 && !L<Foreign>.ptrs, "no gp_fields: the C++ size");

const std::vector<int64_t> go_caps_string = {1,   2,   4,    8,    16,   32,   71,  143,
                                             303, 591, 1023, 1535, 2560, 3584, 5120};
const std::vector<int64_t> go_caps_ptr = {1,   2,   4,    8,    16,   32,   64,   143,
                                          287, 607, 1023, 1535, 2303, 3071, 4095, 6144};
const std::vector<int64_t> go_caps_slice = {1,   2,   4,    8,    16,   37,   74,   170,
                                            341, 682, 1135, 1706, 2389, 3413, 4778, 6485};
const std::vector<int64_t> go_caps_tail = {1,   2,   4,   8,    16,   32,   64,   128,
                                           256, 512, 848, 1280, 1792, 2560, 3584, 5120};
const std::vector<int64_t> go_caps_tail4 = {1,   2,   4,   8,    16,   32,   64,   128,
                                            256, 512, 848, 1280, 1792, 2560, 3584, 5120};
const std::vector<int64_t> go_caps_3byte = {2,   5,    10,   21,   42,   85,   170, 341,
                                            682, 1066, 1621, 2261, 3157, 4522, 6144};
const std::vector<int64_t> go_caps_any = {1,   2,   4,    8,    16,   32,   71,  143,
                                          303, 591, 1023, 1535, 2560, 3584, 5120};
const std::vector<int64_t> go_caps_pair = {1,   2,   4,    8,    16,   32,   71,  143,
                                           303, 591, 1023, 1535, 2560, 3584, 5120};

// grow_caps (above) lists the capacities at each reallocation of appends one at a time.
void test_go_caps() {
    int64_t reallocs = 0;
    bool ok = false;
    CHECK(grow_caps<gp::string>(5000, &reallocs, &ok) == go_caps_string);
    CHECK(grow_caps<gp::any>(5000, &reallocs, &ok) == go_caps_any);
    CHECK(grow_caps<gp::ptr<int64_t>>(5000, &reallocs, &ok) == go_caps_ptr);
    CHECK(grow_caps<gp::slice<int64_t>>(5000, &reallocs, &ok) == go_caps_slice);
    CHECK(grow_caps<GoNamed>(5000, &reallocs, &ok) == go_caps_slice);
    CHECK(grow_caps<GoPair>(5000, &reallocs, &ok) == go_caps_pair);
    CHECK(grow_caps<GoTail>(5000, &reallocs, &ok) == go_caps_tail);
    CHECK(grow_caps<GoTail4>(5000, &reallocs, &ok) == go_caps_tail4);
    CHECK(grow_caps<gp::array<int8_t, 3>>(5000, &reallocs, &ok) == go_caps_3byte);
    CHECK(ok);

    // Zero-size elements: cap == len, and (nearly) no reallocation. GoZero is a zero-size Go
    // type that is not an empty class in C++ (it has real bytes, which a store writes), GoZero3
    // an array of such, GoMarker the marker struct{ _ [0]func() }.
    for (int pass = 0; pass < 2; pass++) {
        gp::slice<Empty> e;
        gp::slice<GoZero> z;
        gp::slice<GoZero3> z3;
        gp::slice<GoMarker> mk;
        int64_t moves = 0;
        for (int64_t i = 0; i < 300'000; i++) {
            GoZero* before = z.gp_data();
            e = gp::append(std::move(e), Empty{});
            z = gp::append(std::move(z), GoZero{}, GoZero{});
            z3 = gp::append(std::move(z3), GoZero3{});
            mk = gp::append(std::move(mk), GoMarker{});
            moves += z.gp_data() != before;
            if (i < 100 || i % 9973 == 0) {
                CHECK_EQ(gp::cap(e), i + 1);
                CHECK_EQ(gp::cap(z), 2 * (i + 1));
                CHECK_EQ(gp::cap(z3), i + 1);
                CHECK_EQ(gp::cap(mk), i + 1);
            }
        }
        CHECK(moves < 64); // the block is bigger than the capacity: amortized O(1)
        CHECK_EQ(gp::len(e), INT64_C(300000));
        CHECK(e.gp_data() == gp::detail::empty_data<Empty>());
        gp::slice<Empty> e2 = e;
        e = gp::append_slice(std::move(e), e2);
        CHECK(gp::len(e) == 600'000 && gp::cap(e) == 600'000);
        gp::clear(e);
        CHECK_EQ(gp::copy(gp::sub(e, 1), e), INT64_C(599'999));
        gp::ptr<Empty> p(e.gp_owner(), &e[5]); // &e[5] has an owner: not nil
        CHECK(p != nullptr);
        gp::clear(z);
        CHECK_EQ(gp::copy(gp::sub(z, 1), z), gp::len(z) - 1);
        gp::slice<GoZero> zc = gp::append_slice(gp::slice<GoZero>(), z);
        CHECK(gp::len(zc) == gp::len(z) && gp::cap(zc) == gp::len(z));
        ints t = gp::append(ints(), INT64_C(1));
        CHECK_EQ(gp::cap(t), INT64_C(1));
    }

    // Many elements at once: newLen above twice the old capacity is taken, then rounded up to
    // the size class (and, with pointers over 512 bytes, with the 8-byte header).
    CHECK_EQ(gp::cap(gp::append_slice(ints(), gp::make_slice<int64_t>(5))), INT64_C(6));
    CHECK_EQ(gp::cap(gp::append_slice(ints(), gp::make_slice<int64_t>(33))), INT64_C(36));
    CHECK_EQ(gp::cap(gp::append_slice(strs(), gp::make_slice<gp::string>(33))), INT64_C(35));
    CHECK_EQ(gp::cap(gp::append_slice(strs(), gp::make_slice<gp::string>(32))), INT64_C(32));
    CHECK_EQ(gp::cap(gp::append_slice(bytes(), gp::make_slice<uint8_t>(513))), INT64_C(576));
    CHECK_EQ(gp::cap(gp::append_slice(gp::slice<gp::any>(), gp::make_slice<gp::any>(513))),
             INT64_C(591));

    // roundup_size(size, noscan), Go's roundupsize.
    CHECK_EQ(gp::detail::roundup_size(0, true), UINT64_C(0));
    CHECK_EQ(gp::detail::roundup_size(1, true), UINT64_C(8));
    CHECK_EQ(gp::detail::roundup_size(513, true), UINT64_C(576));
    CHECK_EQ(gp::detail::roundup_size(32768, true), UINT64_C(32768));
    CHECK_EQ(gp::detail::roundup_size(32769, true), UINT64_C(40960));
    CHECK_EQ(gp::detail::roundup_size(512, false), UINT64_C(512)); // no header up to 512
    CHECK_EQ(gp::detail::roundup_size(513, false), UINT64_C(568)); // 521 -> 576, less the header
    CHECK_EQ(gp::detail::roundup_size(1016, false), UINT64_C(1016));
    CHECK_EQ(gp::detail::roundup_size(1017, false), UINT64_C(1144)); // 1025 -> 1152
    CHECK_EQ(gp::detail::roundup_size(32760, false), UINT64_C(32760));
    CHECK_EQ(gp::detail::roundup_size(32761, false), UINT64_C(32768)); // large: pages, no header

    // The conversions: the size class of the length (runes: of 4 * count, in runes); a constant
    // string is exact.
    auto str = [](int64_t n) {
        return gp::string::gp_copy(std::string(static_cast<size_t>(n), 'x'));
    };
    const std::pair<int64_t, int64_t> byte_caps[] = {
        {1, 8}, {8, 8}, {9, 16}, {32, 32}, {33, 48}, {100, 112}, {600, 640}, {5000, 5376}};
    for (auto [n, c] : byte_caps) {
        CHECK_EQ(gp::cap(gp::to_bytes(str(n))), c);
        CHECK_EQ(gp::cap(gp::to_bytes_const(str(n))), n);
    }
    const std::pair<int64_t, int64_t> rune_caps[] = {{1, 2},   {5, 6},     {8, 8},    {9, 12},
                                                     {33, 36}, {100, 104}, {600, 672}};
    for (auto [n, c] : rune_caps) {
        CHECK_EQ(gp::cap(gp::to_runes(str(n))), c);
        CHECK_EQ(gp::cap(gp::to_runes_const(str(n))), n);
        CHECK_EQ(gp::len(gp::to_runes(str(n))), n);
    }
    gp::slice<uint8_t> hello = gp::to_bytes("hello"_gs);
    CHECK(hello[4] == 'o' && hello.gp_data()[5] == 0); // the spare room is zeroed
    CHECK_EQ(gp::cap(gp::to_bytes(""_gs)), INT64_C(0));
    CHECK_EQ(gp::cap(gp::to_runes_const(""_gs)), INT64_C(0));
}

// --- copy and clear ---

void test_copy_clear() {
    ints k = gp::slice_of<int64_t>({1, 2, 3, 4, 5});
    CHECK_EQ(gp::copy(gp::sub(k, 1), k), INT64_C(4)); // forward overlap
    CHECK(same(k, {1, 1, 2, 3, 4}));
    k = gp::slice_of<int64_t>({1, 2, 3, 4, 5});
    CHECK_EQ(gp::copy(k, gp::sub(k, 2)), INT64_C(3)); // backward overlap
    CHECK(same(k, {3, 4, 5, 4, 5}));
    CHECK_EQ(gp::copy(k, k), INT64_C(5));
    CHECK_EQ(gp::copy(ints(), k), INT64_C(0));

    // Non-trivial elements, overlapping both ways.
    strs s = gp::slice_of<gp::string>({"a"_gs, "b"_gs, "c"_gs, "d"_gs});
    CHECK_EQ(gp::copy(gp::sub(s, 1), s), INT64_C(3));
    CHECK(elems(s) == std::vector<gp::string>({"a"_gs, "a"_gs, "b"_gs, "c"_gs}));
    s = gp::slice_of<gp::string>({"a"_gs, "b"_gs, "c"_gs, "d"_gs});
    CHECK_EQ(gp::copy(s, gp::sub(s, 1)), INT64_C(3));
    CHECK(elems(s) == std::vector<gp::string>({"b"_gs, "c"_gs, "d"_gs, "d"_gs}));

    // copy(dst []byte, src string)
    bytes b = gp::make_slice<uint8_t>(3);
    CHECK_EQ(gp::copy(b, "hello"_gs), INT64_C(3));
    CHECK_EQ(gp::copy(gp::sub(b, 1), "E"_gs), INT64_C(1));
    CHECK(elems(b) == std::vector<uint8_t>({'h', 'E', 'l'}));
    CHECK_EQ(gp::copy(b, ""_gs), INT64_C(0));

    // clear zeroes len elements, not cap.
    ints c = gp::slice_of<int64_t>({1, 2, 3, 4});
    gp::clear(gp::sub(c, 1, 2));
    CHECK(same(c, {1, 0, 3, 4}));
    strs cs = gp::slice_of<gp::string>({"x"_gs, "y"_gs});
    gp::clear(cs);
    CHECK(cs[0] == ""_gs && cs[1] == ""_gs);
    gp::slice<Plain> cp = gp::slice_of<Plain>({Plain{1, 2.5, true}});
    gp::clear(cp);
    CHECK(cp[0].a == 0 && cp[0].b == 0.0 && !cp[0].c);
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
        gp::slice<Tracked> t = gp::make_slice<Tracked>(3, 5);
        CHECK_EQ(live, INT64_C(5)); // every element of the array is a constructed zero value
        for (int64_t i = 0; i < 20; i++) {
            t = gp::append(t, Tracked(i));
        }
        gp::slice<Tracked> u = gp::sub(t, 2, 4);
        t = nullptr;
        CHECK_EQ(u[0].v, INT64_C(0));
        gp::clear(u);
        gp::slice<Tracked> lit = gp::slice_of<Tracked>({Tracked(1), Tracked(2)});
        CHECK_EQ(lit[1].v, INT64_C(2));
    }
    CHECK_EQ(live, INT64_C(0));
    CHECK_EQ(bad_destroys, INT64_C(0));

    // A pointer to an element keeps the array alive after the slice goes.
    ints s = gp::slice_of<int64_t>({1, 2, 3});
    gp::ptr<int64_t> p = gp::ptr<int64_t>(s.gp_owner(), &s[1]);
    gp::weak_ptr<int64_t> w = gp::weak_make(p);
    s = nullptr;
    *p += 40;
    CHECK_EQ(*p, INT64_C(42));
    CHECK(gp::weak_value(w) == p);
    p = nullptr;
    CHECK(gp::weak_value(w) == nullptr); // the array is freed with the last pointer

    // ... also through a sub-slice, and after an append moved the slice to a new array.
    ints a = gp::slice_of<int64_t>({4, 5});
    gp::ptr<int64_t> q = gp::ptr<int64_t>(gp::sub(a, 1).gp_owner(), &gp::sub(a, 1)[0]);
    a = gp::append(a, INT64_C(6));
    *q = 50;
    CHECK(same(a, {4, 5, 6}));
    CHECK_EQ(*q, INT64_C(50));

    // Elements that own things: strings, pointers, nested slices are freed with the array
    // (LeakSanitizer checks there's nothing left in the asan mode).
    gp::ptr<int64_t> target = gp::new_<int64_t>(7);
    gp::weak_ptr<int64_t> wt = gp::weak_make(target);
    gp::slice<gp::ptr<int64_t>> ps = gp::slice_of<gp::ptr<int64_t>>({target, nullptr});
    target = nullptr;
    CHECK(gp::weak_value(wt) != nullptr);
    ps = gp::append(ps, ps[0], ps[0]);
    ps = nullptr;
    CHECK(gp::weak_value(wt) == nullptr);

    ints inner = gp::slice_of<int64_t>({1});
    gp::weak_ptr<int64_t> wi = gp::weak_make(gp::ptr<int64_t>(inner.gp_owner(), &inner[0]));
    gp::slice<ints> outer = gp::slice_of<ints>({inner, inner});
    inner = nullptr;
    outer = gp::append(outer, outer[1]);
    CHECK(gp::weak_value(wi) != nullptr);
    outer = nullptr;
    CHECK(gp::weak_value(wi) == nullptr);

    strs ss = gp::slice_of<gp::string>({gp::string::gp_copy("heap")});
    ss = gp::append(ss, ss[0], gp::string::gp_copy("more"));
    ss = gp::sub(ss, 1);
    CHECK(ss[0] == "heap"_gs && ss[1] == "more"_gs);

    // An assignment reads the new value before it drops the old array: n's array holds the
    // only reference to the array of n[0].kids.
    gp::slice<Node> n = gp::make_slice<Node>(1);
    n[0].kids = gp::slice_of<Node>({Node{nullptr, 5}});
    n = n[0].kids;
    CHECK(gp::len(n) == 1 && n[0].v == 5);
    n = std::move(n[0].kids); // nil: n[0] has no kids
    CHECK(n == nullptr);

    // A slice of a package-level array shares the static owner, so &s[i] has an owner.
    ints g(gp::static_owner(), global_arr, 3, 3);
    gp::ptr<int64_t> gp0 = gp::ptr<int64_t>(g.gp_owner(), &g[2]);
    *gp0 = 30;
    CHECK_EQ(global_arr[2], INT64_C(30));
    gp::weak_ptr<int64_t> wg = gp::weak_make(gp0);
    g = nullptr;
    gp0 = nullptr;
    CHECK(gp::weak_value(wg) != nullptr); // never expires
}

// Stores whose source array is owned only by the element that the store overwrites: q[0].kids
// is the only owner of its array, and storing into q[0] drops it. Without may_free (slice.hpp)
// each case read freed memory (ASan: heap-use-after-free). The values are real Go's.
std::vector<int64_t> vals(const gp::slice<Node>& q) {
    std::vector<int64_t> v;
    for (int64_t i = 0; i < gp::len(q); i++) {
        v.push_back(q[i].v);
    }
    return v;
}

gp::slice<Node> with_kids(std::vector<int64_t> kids, std::vector<int64_t> rest) {
    gp::slice<Node> k;
    for (int64_t v : kids) {
        k = gp::append(k, Node{nullptr, v});
    }
    gp::slice<Node> q = gp::append(gp::slice<Node>(), Node{k, 0});
    for (int64_t v : rest) {
        q = gp::append(q, Node{nullptr, v});
    }
    return q;
}

void test_self_free() {
    gp::slice<Node> q = with_kids({5, 6, 7}, {});
    q = gp::append_slice(gp::sub(q, 0, 0), q[0].kids);
    CHECK(vals(q) == std::vector<int64_t>({5, 6, 7}));

    q = with_kids({1}, {5});
    CHECK_EQ(gp::copy(gp::sub(q, 0, 1), q[0].kids), INT64_C(1));
    CHECK(vals(q) == std::vector<int64_t>({1, 5}));

    q = with_kids({3, 5, 6, 7}, {1, 2});
    CHECK_EQ(gp::copy(q, q[0].kids), INT64_C(3));
    CHECK(vals(q) == std::vector<int64_t>({3, 5, 6}));

    q = with_kids({5, 8}, {});
    q = gp::append(gp::sub(q, 0, 0), q[0].kids[0]);
    CHECK(vals(q) == std::vector<int64_t>({5}));

    q = with_kids({5, 8}, {});
    q = gp::append(gp::sub(q, 0, 0), q[0].kids[1], q[0].kids[0]);
    CHECK(vals(q) == std::vector<int64_t>({8, 5}));

    // clear of a slice that an element of its own array holds (a cycle, broken by the clear).
    gp::slice<Node> k = gp::append(gp::slice<Node>(), Node{nullptr, 9});
    k[0].kids = k;
    gp::clear(k[0].kids);
    CHECK(k[0].v == 0 && k[0].kids == nullptr);
}

// --- freeing long chains (detail::release) ---

int64_t freed = 0;

// A list whose links hold slices of pointers to the next link, and one whose links hold the
// next link by value inside a slice.
struct PLink {
    gp::slice<gp::ptr<PLink>> next;
    ~PLink() { freed++; }
};
struct VLink {
    gp::slice<VLink> next;
    int64_t pad = 0;
    ~VLink() { freed++; }
};

bool nothing_pending() {
    return gp::detail::release_tls.gp_depth == 0 && gp::detail::release_tls.gp_pending == nullptr;
}

void test_chains() {
    const int64_t n = 200'000;
    freed = 0;
    {
        gp::ptr<PLink> head;
        for (int64_t i = 0; i < n; i++) {
            gp::ptr<PLink> l = gp::new_<PLink>();
            l->next = gp::slice_of<gp::ptr<PLink>>({head});
            head = std::move(l);
        }
    } // dropped at the end of the scope
    CHECK_EQ(freed, n);
    CHECK(nothing_pending());

    freed = 0;
    gp::slice<VLink> top;
    for (int64_t i = 0; i < n; i++) {
        gp::slice<VLink> s = gp::make_slice<VLink>(1);
        s[0].next = std::move(top);
        top = std::move(s);
    }
    int64_t made = freed; // nothing was freed while building
    top = nullptr;        // dropped by an assignment
    CHECK_EQ(made, INT64_C(0));
    CHECK_EQ(freed, n);
    CHECK(nothing_pending());

    // A slice in the middle of a chain: the rest is freed when the last slice of it goes.
    freed = 0;
    gp::ptr<PLink> h;
    for (int64_t i = 0; i < n; i++) {
        gp::ptr<PLink> l = gp::new_<PLink>();
        l->next = gp::slice_of<gp::ptr<PLink>>({h});
        h = std::move(l);
    }
    gp::slice<gp::ptr<PLink>> mid = h->next;
    h = nullptr;
    CHECK_EQ(freed, INT64_C(1));
    mid = nullptr;
    CHECK_EQ(freed, n);
    CHECK(nothing_pending());
}

void test_adl() {
    using gpkg_adl::P;
    gp::slice<P> s;
    s = gp::append(s, P{1});
    s = gp::append(s, P{2}, P{3});
    s = gp::append_slice(s, gp::sub(s, 1));
    s = gp::sub(s, 1);
    CHECK_EQ(gp::copy(s, gp::sub(s, 1)), INT64_C(3));
    CHECK_EQ(gp::len(s), INT64_C(4));
    CHECK(s[0].v == 3 && s[1].v == 2 && s[2].v == 3 && s[3].v == 3);
    CHECK_EQ(gpkg_adl::hijacked, INT64_C(0));
}

// --- keyed literals, element pointers, range and the string conversions (step 3.5b) ---

void test_keyed_range_conv() {
    // []int{5: 1, 2, 1: 7}: length 7, the rest zero, evaluated in source order.
    ints k = gp::slice_keyed<int64_t>(7, {{5, 1}, {6, 2}, {1, 7}});
    CHECK(same(k, {0, 7, 0, 0, 0, 1, 2}));
    CHECK_EQ(gp::cap(k), int64_t(7));
    strs ks = gp::slice_keyed<gp::string>(3, {{2, "x"_gs}});
    CHECK(ks[0] == ""_gs && ks[1] == ""_gs && ks[2] == "x"_gs);
    CHECK_EQ(panic_text([] { gp::slice_keyed<int64_t>(2, {{2, 1}}); }),
             std::string("runtime error: index out of range [2] with length 2"));

    // &s[i] of a slice that is not a variable keeps its array alive.
    gp::ptr<int64_t> p = gp::elem_ptr(gp::slice_of<int64_t>({4, 5, 6}), 1);
    CHECK_EQ(*p, int64_t(5));
    *p = 9;
    CHECK_EQ(*p, int64_t(9));
    CHECK_EQ(panic_text([] { gp::elem_ptr(ints(), 0); }),
             std::string("runtime error: index out of range [0] with length 0"));

    // range: the original length and array; later elements as they are when reached.
    ints s = gp::slice_of<int64_t>({1, 2, 3});
    std::vector<int64_t> seen;
    for (auto it = gp::range_slice(s); gp::range_next(it);) {
        seen.push_back(it.gp_i * 10 + it.gp_v());
        if (it.gp_i == 0) {
            s[2] = 7;                      // seen: the copy shares the array
            s = gp::append(s, 4);          // not seen: the range has the old length
            s = gp::slice_of<int64_t>({}); // not seen either
        }
    }
    CHECK(seen == (std::vector<int64_t>{1, 12, 27}));
    int n = 0;
    for (auto it = gp::range_slice(ints()); gp::range_next(it);) {
        n++;
    }
    CHECK_EQ(int64_t(n), int64_t(0));

    // []byte(s), string(b): copies.
    bytes b = gp::to_bytes("héllo"_gs);
    CHECK_EQ(gp::len(b), int64_t(6));
    CHECK(b[1] == 0xC3 && b[2] == 0xA9);
    b[0] = 'j';
    CHECK(gp::to_string(b) == "jéllo"_gs);
    gp::string from = gp::to_string(b);
    b[0] = 'x';
    CHECK(from == "jéllo"_gs);
    CHECK(gp::to_bytes(""_gs) != nullptr && gp::len(gp::to_bytes(""_gs)) == 0);
    CHECK(gp::to_string(bytes()) == ""_gs);

    // []rune(s): decoded like range over s (an invalid byte is U+FFFD).
    gp::slice<int32_t> r = gp::to_runes("a\xffé\xe2\x82世"_gs);
    CHECK_EQ(gp::len(r), int64_t(6));
    CHECK(r[0] == 'a' && r[1] == 0xFFFD && r[2] == 0xE9 && r[3] == 0xFFFD && r[4] == 0xFFFD &&
          r[5] == 0x4E16);
    CHECK(gp::to_runes(""_gs) != nullptr && gp::len(gp::to_runes(""_gs)) == 0);

    // string(runes): invalid code points are U+FFFD.
    gp::slice<int32_t> rs = gp::slice_of<int32_t>({104, -1, 0xD800, 0x110000, 0xE9, 0x10FFFF});
    CHECK(gp::to_string(rs) == "h\xef\xbf\xbd\xef\xbf\xbd\xef\xbf\xbd\xc3\xa9\xf4\x8f\xbf\xbf"_gs);
    CHECK(gp::to_string(gp::slice<int32_t>()) == ""_gs);
}

void run_tests() {
    test_nil_and_empty();
    test_make();
    test_index();
    test_sub();
    test_aliasing();
    test_append();
    test_growth();
    test_go_caps();
    test_copy_clear();
    test_lifetimes();
    test_self_free();
    test_chains();
    test_adl();
    test_keyed_range_conv();
    if (failures > 0) {
        std::string msg =
            "FAIL: " + std::to_string(failures) + " of " + std::to_string(checks) + " checks\n";
        gp::os::write(gp::os::stderr_fd, msg.data(), msg.size());
        gp::os::exit(1);
    }
    gp::fmt::Printf("ok: %d checks\n"_gs, int64_t(checks));
}

void no_init() {}

} // namespace

int main(int argc, char** argv) {
    return gp::run_main(argc, argv, &no_init, &run_tests);
}
