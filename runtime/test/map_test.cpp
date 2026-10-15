// The unit tests of gp::map (gocpp/map.hpp) and of key hashing (gocpp/hash.hpp).
// runtime/cxx_test.go compiles this file with the runtime's .cpp files, with g++ and clang++,
// at -O0, at -O2 and with ASan + UBSan (LeakSanitizer included), and runs it like
// test/unit.cpp. It prints "ok: N checks" and exits 0, or prints each failed check and exits 1.
// The Go behavior it checks is in runtime/testdata/diff/main.go (mapCase) too, where real Go
// runs it.
#include <gocpp/rt.hpp>
#include <gocpp/os.hpp>

#include <algorithm>
#include <chrono>
#include <complex>
#include <concepts>
#include <cstdint>
#include <limits>
#include <memory>
#include <string>
#include <type_traits>
#include <utility>
#include <vector>

// A user's package (a namespace like generated code's gpkg_main) with Go functions whose names
// are also the names of the runtime's helpers. The runtime's calls are qualified, so
// argument-dependent lookup for a map with a user's key type never picks these (test_adl).
namespace gpkg_adl {

struct K {
    int64_t v = 0;
    bool operator==(const K&) const = default;
    std::size_t gp_hash() const { return gp::hash_fields(v); }
};

int64_t hijacked = 0;

template <typename M, typename X>
auto map_find(M&, const X&, std::size_t) {
    hijacked++;
    return 0;
}
template <typename M, typename X>
int64_t& map_slot(M&, const X&) {
    hijacked++;
    static int64_t x;
    return x;
}
template <typename X>
bool self_equal(const X&, const X&) {
    hijacked++;
    return false;
}
template <typename X>
std::size_t hash_of(const X&) {
    hijacked++;
    return 0;
}
template <typename... X>
std::size_t hash_fields(const X&...) {
    hijacked++;
    return 0;
}
template <typename M>
int64_t len(const M&) {
    hijacked++;
    return 0;
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

const double nan = std::numeric_limits<double>::quiet_NaN();
const std::string nil_map_panic = "assignment to entry in nil map";

using imap = gp::map<int64_t, int64_t>;
using smap = gp::map<gp::string, int64_t>;
using fmap = gp::map<double, int64_t>;

// keys: the keys of m, through a range, sorted.
template <typename K, typename V>
std::vector<K> keys(const gp::map<K, V>& m) {
    std::vector<K> ks;
    for (auto it = gp::range_map(m); gp::range_next(it);) {
        ks.push_back(it.gp_key());
    }
    std::sort(ks.begin(), ks.end());
    return ks;
}

// count: the number of iterations of a range over m.
template <typename K, typename V>
int64_t count(const gp::map<K, V>& m) {
    int64_t n = 0;
    for (auto it = gp::range_map(m); gp::range_next(it);) {
        n++;
    }
    return n;
}

// Under ASan the chains and the stress test are smaller: ASan's shadow memory and quarantine
// made them take about 600 MB per compiler (TestCXX runs both at once). 50,000 links still
// overflow the stack at -O0 without detail::release in ~map (checked by hand).
#if defined(__SANITIZE_ADDRESS__)
constexpr bool asan = true;
#elif defined(__has_feature)
#if __has_feature(address_sanitizer)
constexpr bool asan = true;
#else
constexpr bool asan = false;
#endif
#else
constexpr bool asan = false;
#endif

constexpr int64_t chain_nodes = asan ? 50'000 : 200'000;
constexpr int64_t stress_ops = asan ? 100'000 : 1'000'000;

bool nothing_pending() {
    return gp::detail::release_tls.gp_depth == 0 && gp::detail::release_tls.gp_pending == nullptr;
}

// --- layout, zero values, the type's rules ---

// A map accepts incomplete key and value types: Node is incomplete where its members are
// declared.
struct Node {
    gp::map<int64_t, Node> kids;
    gp::map<gp::string, gp::ptr<Node>> named;
    gp::map<gp::ptr<Node>, bool> seen;
    int64_t v = 0;
};

constinit imap global_nil;

static_assert(sizeof(imap) == 16);

// Keys of named types (gp::newtype, step 3.9) are replaced like those of their underlying
// types: never for integers and pointers, always for floats and strings.
struct NamedInt : gp::newtype<NamedInt, int64_t> {
    using gp::newtype<NamedInt, int64_t>::newtype;
};
struct NamedPtr : gp::newtype<NamedPtr, gp::ptr<int64_t>> {
    using gp::newtype<NamedPtr, gp::ptr<int64_t>>::newtype;
};
struct NamedFloat : gp::newtype<NamedFloat, double> {
    using gp::newtype<NamedFloat, double>::newtype;
};
struct NamedStr : gp::newtype<NamedStr, gp::string> {
    using gp::newtype<NamedStr, gp::string>::newtype;
};
static_assert(!gp::detail::key_update<NamedInt> && !gp::detail::key_update<NamedPtr>);
static_assert(gp::detail::key_update<NamedFloat> && gp::detail::key_update<NamedStr>);
static_assert(sizeof(gp::map<gp::string, Node>) == 16);
static_assert(alignof(imap) == 8);
static_assert(!std::equality_comparable<imap>, "Go compares maps only with nil");
static_assert(requires(imap m) { m == nullptr; });
static_assert(requires(imap m) { m != nullptr; });
static_assert(std::is_nothrow_move_constructible_v<imap>);
static_assert(std::is_nothrow_copy_constructible_v<imap>);
static_assert(!std::is_convertible_v<imap, bool>);
static_assert(std::is_same_v<decltype(gp::get(std::declval<imap>(), 1)), int64_t>,
              "a read gives a copy, never a reference into the table");
static_assert(std::is_same_v<decltype(gp::ref(std::declval<imap>(), 1)), int64_t&>);
static_assert(
    std::is_same_v<decltype(gp::lookup(std::declval<imap>(), 1)), std::tuple<int64_t, bool>>);

void test_nil() {
    imap m;
    CHECK(m == nullptr);
    CHECK(global_nil == nullptr);
    CHECK_EQ(gp::len(m), INT64_C(0));
    CHECK_EQ(gp::get(m, 1), INT64_C(0));
    auto [v, ok] = gp::lookup(m, 1);
    CHECK(v == 0 && !ok);
    gp::erase(m, 1); // no-ops
    gp::clear(m);
    CHECK_EQ(count(m), INT64_C(0));
    CHECK_EQ(panic_text([&] { gp::set(m, 1, 2); }), nil_map_panic);
    CHECK_EQ(panic_text([&] { gp::ref(m, 1) += 1; }), nil_map_panic);
    CHECK_EQ(panic_text([&] { gp::ref(m, 1)++; }), nil_map_panic);
    CHECK(m == nullptr);

    gp::map<gp::string, gp::string> ns;
    CHECK(gp::get(ns, "a"_gs) == ""_gs);
    CHECK_EQ(panic_text([&] { gp::set(ns, "a"_gs, "b"_gs); }), nil_map_panic);

    // make and an empty literal give empty maps that are not nil.
    imap e = gp::make_map<int64_t, int64_t>();
    CHECK(e != nullptr);
    CHECK_EQ(gp::len(e), INT64_C(0));
    imap lit = gp::map_of<int64_t, int64_t>({});
    CHECK(lit != nullptr);
    CHECK_EQ(gp::len(lit), INT64_C(0));
    e = nullptr;
    CHECK(e == nullptr);
}

void test_make() {
    // A hint only reserves room: negative counts as 0, a huge one is ignored (as in Go).
    imap a = gp::make_map<int64_t, int64_t>(100);
    imap b = gp::make_map<int64_t, int64_t>(-1);
    imap c = gp::make_map<int64_t, int64_t>(INT64_MAX);
    imap d = gp::make_map<int64_t, int64_t>(UINT64_MAX);
    imap z = gp::make_map<int64_t, int64_t>(uint8_t(0));
    // A big hint that Go accepts (it allocates as the map fills): only 2^20 entries reserved.
    gp::map<gp::string, int64_t> big = gp::make_map<gp::string, int64_t>(INT64_C(1) << 40);
    CHECK(big.gp_get()->gp_t.bucket_count() <= 2 * gp::detail::max_map_reserve);
    gp::set(big, "x"_gs, 1);
    CHECK_EQ(gp::len(big), INT64_C(1));
    for (imap* m : {&a, &b, &c, &d, &z}) {
        CHECK(*m != nullptr);
        CHECK_EQ(gp::len(*m), INT64_C(0));
        gp::set(*m, 7, 8);
        CHECK_EQ(gp::get(*m, 7), INT64_C(8));
    }
    CHECK(a.gp_get()->gp_t.bucket_count() >= 100);
}

void test_basic() {
    imap m = gp::make_map<int64_t, int64_t>();
    gp::set(m, 1, 10);
    gp::set(m, 2, 20);
    CHECK_EQ(gp::len(m), INT64_C(2));
    CHECK_EQ(gp::get(m, 1), INT64_C(10));
    CHECK_EQ(gp::get(m, 3), INT64_C(0));
    CHECK_EQ(gp::len(m), INT64_C(2)); // reads never insert
    auto [v, ok] = gp::lookup(m, 2);
    CHECK(v == 20 && ok);
    auto [v3, ok3] = gp::lookup(m, 3);
    CHECK(v3 == 0 && !ok3);
    CHECK_EQ(gp::len(m), INT64_C(2));

    gp::set(m, 1, 11); // overwrite
    CHECK_EQ(gp::get(m, 1), INT64_C(11));
    CHECK_EQ(gp::len(m), INT64_C(2));

    // m[k] op= v and m[k]++ insert the zero value first.
    gp::ref(m, 5) += 3;
    gp::ref(m, 5)++;
    gp::ref(m, 1) *= 2;
    CHECK_EQ(gp::get(m, 5), INT64_C(4));
    CHECK_EQ(gp::get(m, 1), INT64_C(22));
    CHECK_EQ(gp::len(m), INT64_C(3));
    // C++17 evaluates the right side of += first, so a read of the map there sees the old
    // state, as Go's two-phase assignment does.
    gp::ref(m, 6) += gp::len(m);
    CHECK_EQ(gp::get(m, 6), INT64_C(3));

    // delete
    gp::erase(m, 6);
    gp::erase(m, 6);
    gp::erase(m, 99);
    CHECK_EQ(gp::len(m), INT64_C(3));
    CHECK(!std::get<1>(gp::lookup(m, 6)));
    CHECK(keys(m) == std::vector<int64_t>({1, 2, 5}));

    // A map is a reference: a copy sees the writes.
    imap alias = m;
    gp::set(alias, 9, 90);
    CHECK_EQ(gp::get(m, 9), INT64_C(90));
    gp::clear(m);
    CHECK_EQ(gp::len(alias), INT64_C(0));
    gp::set(m, 1, 1); // still usable after clear
    CHECK_EQ(gp::len(alias), INT64_C(1));

    // The key and value convert to K and V (an int literal for an int64_t key, a smaller type).
    gp::map<uint8_t, gp::string> bm = gp::make_map<uint8_t, gp::string>();
    gp::set(bm, 255, "x"_gs);
    CHECK(gp::get(bm, uint8_t(255)) == "x"_gs);

    // Self-assignment and moves.
    imap s = m;
    imap& same = s;
    s = same;
    CHECK_EQ(gp::len(s), INT64_C(1));
    imap t = std::move(s);
    CHECK(s == nullptr);
    CHECK_EQ(gp::len(t), INT64_C(1));
    imap& same_t = t;
    t = std::move(same_t);
    CHECK_EQ(gp::len(t), INT64_C(1));
}

void test_literal() {
    smap m = gp::map_of<gp::string, int64_t>({{"a"_gs, 1}, {"b"_gs, 2}, {"c"_gs, 3}});
    CHECK_EQ(gp::len(m), INT64_C(3));
    CHECK_EQ(gp::get(m, "b"_gs), INT64_C(2));
    // Equal non-constant keys: stored in source order, so the later one wins.
    int64_t k = 4;
    imap d = gp::map_of<int64_t, int64_t>({{k, 1}, {2, 2}, {k, 3}});
    CHECK_EQ(gp::len(d), INT64_C(2));
    CHECK_EQ(gp::get(d, 4), INT64_C(3));
    // The stored key is the later one too (Go's mapassign updates the key).
    fmap z = gp::map_of<double, int64_t>({{-0.0, 1}, {0.0, 2}});
    CHECK_EQ(gp::len(z), INT64_C(1));
    CHECK(keys(z).size() == 1 && !std::signbit(keys(z)[0]));
}

void test_floats() {
    // NaN != NaN: each store adds an entry, lookups never find one, delete can't remove one,
    // a range visits each, clear removes them.
    fmap m = gp::make_map<double, int64_t>();
    gp::set(m, nan, 1);
    gp::set(m, nan, 2);
    gp::ref(m, nan) += 4;
    CHECK_EQ(gp::len(m), INT64_C(3));
    CHECK_EQ(gp::get(m, nan), INT64_C(0));
    CHECK(!std::get<1>(gp::lookup(m, nan)));
    gp::erase(m, nan);
    CHECK_EQ(gp::len(m), INT64_C(3));
    gp::set(m, 1.5, 100);
    int64_t sum = 0, nans = 0;
    for (auto it = gp::range_map(m); gp::range_next(it);) {
        sum += it.gp_value();
        nans += it.gp_key() != it.gp_key();
    }
    CHECK_EQ(sum, INT64_C(107));
    CHECK_EQ(nans, INT64_C(3));
    // A delete of another key during a range: the NaN entries are still visited.
    sum = 0;
    nans = 0;
    for (auto it = gp::range_map(m); gp::range_next(it);) {
        double k = it.gp_key(); // copied first, as the emitter does: the erase may free it
        int64_t v = it.gp_value();
        gp::erase(m, 1.5);
        sum += k == 1.5 ? 0 : v;
        nans += k != k;
    }
    CHECK_EQ(nans, INT64_C(3));
    CHECK_EQ(sum, INT64_C(7));
    CHECK_EQ(gp::len(m), INT64_C(3));
    gp::clear(m);
    CHECK_EQ(gp::len(m), INT64_C(0));
    CHECK_EQ(count(m), INT64_C(0));

    // +0 and -0 are one key; a store replaces the stored key with the new one.
    gp::set(m, -0.0, 1);
    gp::set(m, 0.0, 2);
    CHECK_EQ(gp::len(m), INT64_C(1));
    CHECK_EQ(gp::get(m, -0.0), INT64_C(2));
    CHECK(!std::signbit(keys(m)[0]));
    gp::ref(m, -0.0)++;
    CHECK(std::signbit(keys(m)[0]));
    CHECK_EQ(gp::get(m, 0.0), INT64_C(3));
    gp::erase(m, 0.0);
    CHECK_EQ(gp::len(m), INT64_C(0));

    // Infinities and ordinary values.
    double inf = std::numeric_limits<double>::infinity();
    gp::set(m, inf, 1);
    gp::set(m, -inf, 2);
    gp::set(m, 0.1 + 0.2, 3);
    CHECK_EQ(gp::get(m, inf), INT64_C(1));
    CHECK_EQ(gp::get(m, -inf), INT64_C(2));
    CHECK_EQ(gp::get(m, 0.30000000000000004), INT64_C(3));
    CHECK_EQ(gp::get(m, 0.3), INT64_C(0));

    // float32 keys, and NaNs with other payloads.
    gp::map<float, int64_t> f = gp::make_map<float, int64_t>();
    gp::set(f, -0.0f, 1);
    gp::set(f, 0.0f, 2);
    gp::set(f, std::numeric_limits<float>::quiet_NaN(), 3);
    gp::set(f, -std::numeric_limits<float>::quiet_NaN(), 4);
    CHECK_EQ(gp::len(f), INT64_C(3));
    CHECK_EQ(gp::get(f, 0.0f), INT64_C(2));

    // The hashes: ±0 equal, each NaN new.
    CHECK(gp::hash_of(0.0) == gp::hash_of(-0.0));
    CHECK(gp::hash_of(0.0f) == gp::hash_of(-0.0f));
    CHECK(gp::hash_of(nan) == gp::hash_of(-nan)); // one hash; the map spreads NaN keys itself
    // ... with its own count: many NaN keys don't share a bucket.
    fmap many = gp::make_map<double, int64_t>();
    for (int64_t i = 0; i < 1000; i++) {
        gp::set(many, nan, i);
    }
    CHECK_EQ(gp::len(many), INT64_C(1000));
    std::size_t biggest = 0;
    auto& t = many.gp_get()->gp_t;
    for (std::size_t b = 0; b < t.bucket_count(); b++) {
        biggest = std::max(biggest, t.bucket_size(b));
    }
    CHECK(biggest < 10);
    CHECK(gp::hash_of(1.0) != gp::hash_of(2.0));

    // Complex keys (Go's complex128, gp::complex128): each part like a float.
    {
        using g128 = gp::complex128;
        gp::map<g128, int64_t> g = gp::make_map<g128, int64_t>();
        gp::set(g, g128(1, -0.0), 1);
        gp::set(g, g128(1, 0.0), 2);
        gp::set(g, g128(nan, 0), 3);
        gp::set(g, g128(nan, 0), 4);
        gp::set(g, g128(0, nan), 5);
        gp::set(g, g128(0, 1), 6);
        gp::ref(g, g128(0, 1)) += 10;
        CHECK_EQ(gp::len(g), INT64_C(5));
        CHECK_EQ(gp::get(g, g128(1, 0)), INT64_C(2));
        CHECK_EQ(gp::get(g, g128(-0.0, 1)), INT64_C(16));
        CHECK_EQ(gp::get(g, g128(nan, 0)), INT64_C(0));
        CHECK_EQ(count(g), INT64_C(5));
        gp::map<gp::complex64, int64_t> g64 = gp::make_map<gp::complex64, int64_t>();
        gp::set(g64, gp::complex64(2, 3), 6);
        CHECK_EQ(gp::get(g64, gp::complex64(2, 3)), INT64_C(6));
        CHECK_EQ(gp::get(g64, gp::complex64(3, 2)), INT64_C(0));
    }
    // std::complex keys (C++ code, Phase 04): the same rules, through real() and imag().
    using c128 = std::complex<double>;
    gp::map<c128, int64_t> c = gp::make_map<c128, int64_t>();
    gp::set(c, c128(1, -0.0), 1);
    gp::set(c, c128(1, 0.0), 2);
    gp::set(c, c128(nan, 0), 3);
    gp::set(c, c128(nan, 0), 4);
    gp::set(c, c128(0, 1), 5);
    CHECK_EQ(gp::len(c), INT64_C(4));
    CHECK_EQ(gp::get(c, c128(1, 0)), INT64_C(2));
    CHECK_EQ(gp::get(c, c128(nan, 0)), INT64_C(0));
    CHECK_EQ(count(c), INT64_C(4));
    CHECK(gp::hash_of(c128(-0.0, 0.0)) == gp::hash_of(c128(0.0, -0.0)));
    CHECK(gp::hash_of(c128(1, 2)) != gp::hash_of(c128(2, 1))); // the parts in order
    gp::map<std::complex<float>, int64_t> c64 = gp::make_map<std::complex<float>, int64_t>();
    gp::set(c64, std::complex<float>(2, 3), 6);
    CHECK_EQ(gp::get(c64, std::complex<float>(2, 3)), INT64_C(6));
}

enum class color : int8_t { red = -1, green, blue };

void test_keys() {
    // Strings by their bytes: equal strings from different storage are one key.
    smap s = gp::make_map<gp::string, int64_t>();
    gp::string built = gp::string::gp_copy("hello");
    gp::set(s, "hello"_gs, 1);
    gp::set(s, built, 2);
    gp::set(s, ""_gs, 3);
    gp::set(s, gp::string(gp::static_bytes, "a\0b", 3), 4);
    gp::set(s, "a"_gs, 5);
    gp::set(s, gp::string(gp::static_bytes, "hello, world", 5), 6);
    CHECK_EQ(gp::len(s), INT64_C(4));
    CHECK_EQ(gp::get(s, "hello"_gs), INT64_C(6));
    CHECK_EQ(gp::get(s, ""_gs), INT64_C(3));
    CHECK_EQ(gp::get(s, gp::string()), INT64_C(3)); // a null data pointer is the empty string
    CHECK_EQ(gp::get(s, gp::string(gp::static_bytes, "a\0b", 3)), INT64_C(4));
    CHECK_EQ(gp::get(s, gp::string(gp::static_bytes, "a\0c", 3)), INT64_C(0));
    CHECK(gp::hash_of("hello"_gs) == gp::hash_of(built));
    // Every length up to a few words, so every tail of hash_bytes runs.
    std::string text = "the quick brown fox jumps over the lazy dog";
    smap prefixes = gp::make_map<gp::string, int64_t>();
    for (std::size_t i = 0; i <= text.size(); i++) {
        gp::set(prefixes, gp::string::gp_copy(std::string_view(text).substr(0, i)),
                static_cast<int64_t>(i));
    }
    CHECK_EQ(gp::len(prefixes), static_cast<int64_t>(text.size() + 1));
    for (std::size_t i = 0; i <= text.size(); i++) {
        CHECK_EQ(gp::get(prefixes, gp::string::gp_copy(std::string_view(text).substr(0, i))),
                 static_cast<int64_t>(i));
    }

    // Pointers by address: two objects with the same value are two keys; the same object
    // through two pointers (or an interior pointer made twice) is one key.
    gp::map<gp::ptr<int64_t>, int64_t> p = gp::make_map<gp::ptr<int64_t>, int64_t>();
    gp::ptr<int64_t> a = gp::new_<int64_t>(5), b = gp::new_<int64_t>(5);
    gp::ptr<int64_t> a2 = a;
    gp::set(p, a, 1);
    gp::set(p, b, 2);
    gp::set(p, a2, 3);
    gp::set(p, nullptr, 4);
    CHECK_EQ(gp::len(p), INT64_C(3));
    CHECK_EQ(gp::get(p, a), INT64_C(3));
    CHECK_EQ(gp::get(p, gp::ptr<int64_t>()), INT64_C(4));
    struct pair2 {
        int64_t x = 0, y = 0;
    };
    gp::ptr<pair2> pp = gp::new_<pair2>();
    gp::map<gp::ptr<int64_t>, int64_t> fp = gp::make_map<gp::ptr<int64_t>, int64_t>();
    gp::set(fp, gp::ptr<int64_t>(pp, &pp->y), 1);
    gp::set(fp, gp::ptr<int64_t>(pp, &pp->y), 2);
    gp::set(fp, gp::ptr<int64_t>(pp, &pp->x), 3);
    CHECK_EQ(gp::len(fp), INT64_C(2));
    // Weak pointers: by the address they were made from.
    gp::map<gp::weak_ptr<int64_t>, int64_t> w = gp::make_map<gp::weak_ptr<int64_t>, int64_t>();
    gp::set(w, gp::weak_make(a), 1);
    gp::set(w, gp::weak_make(a2), 2);
    gp::set(w, gp::weak_make(b), 3);
    CHECK_EQ(gp::len(w), INT64_C(2));
    CHECK_EQ(gp::get(w, gp::weak_make(a)), INT64_C(2));

    // bool, small and unsigned integers, enums.
    gp::map<bool, int64_t> bl = gp::make_map<bool, int64_t>();
    gp::set(bl, true, 1);
    gp::set(bl, false, 2);
    gp::ref(bl, true)++;
    CHECK(gp::get(bl, true) == 2 && gp::get(bl, false) == 2 && gp::len(bl) == 2);
    gp::map<int8_t, int64_t> i8 = gp::make_map<int8_t, int64_t>();
    for (int i = -128; i < 128; i++) {
        gp::set(i8, static_cast<int8_t>(i), i);
    }
    CHECK_EQ(gp::len(i8), INT64_C(256));
    CHECK_EQ(gp::get(i8, int8_t(-128)), INT64_C(-128));
    gp::map<uint64_t, int64_t> u = gp::make_map<uint64_t, int64_t>();
    gp::set(u, UINT64_MAX, 1);
    gp::set(u, 0, 2);
    CHECK_EQ(gp::get(u, UINT64_MAX), INT64_C(1));
    gp::map<color, int64_t> col = gp::make_map<color, int64_t>();
    gp::set(col, color::red, 1);
    gp::set(col, color::blue, 3);
    CHECK_EQ(gp::get(col, color::red), INT64_C(1));
    CHECK_EQ(gp::get(col, color::green), INT64_C(0));

    // A C++ type without gp_hash uses std::hash (C and C++ types as keys, Phase 04).
    gp::map<std::string, int64_t> cs = gp::make_map<std::string, int64_t>();
    gp::set(cs, std::string("x"), 1);
    CHECK_EQ(gp::get(cs, std::string("x")), INT64_C(1));
}

// Structs and arrays as keys, through the gp_hash protocol, as the emitter writes them
// (step 3.8 and 3.6).
struct Point {
    int64_t X = 0;
    int64_t Y = 0;
    bool operator==(const Point&) const = default;
    std::size_t gp_hash() const { return gp::hash_fields(X, Y); }
};

struct Mixed {
    gp::string name;
    double f = 0;
    gp::ptr<int64_t> p;
    bool b = false;
    bool operator==(const Mixed&) const = default;
    std::size_t gp_hash() const { return gp::hash_fields(name, f, p, b); }
};

struct Empty {
    bool operator==(const Empty&) const = default;
    std::size_t gp_hash() const { return gp::hash_fields(); }
};

template <typename T>
concept has_gp_hash = requires(const T& x) { x.gp_hash(); };

// [3]float64: gp::array's own gp_hash.
using Arr3 = gp::array<double, 3>;

// A struct with Go methods named real and imag is hashed by its gp_hash, not as a complex.
struct NotComplex {
    double re = 0, im = 0, extra = 0;
    double real() const { return re; }
    double imag() const { return im; }
    bool operator==(const NotComplex&) const = default;
    std::size_t gp_hash() const { return gp::hash_fields(re, im, extra); }
};

// Nested: a struct holding a struct and an array.
struct Outer {
    Point pt;
    Arr3 arr;
    bool operator==(const Outer&) const = default;
    std::size_t gp_hash() const { return gp::hash_fields(pt, arr); }
};

void test_struct_keys() {
    gp::map<Point, gp::string> m = gp::make_map<Point, gp::string>();
    gp::set(m, Point{1, 2}, "a"_gs);
    gp::set(m, Point{2, 1}, "b"_gs);
    gp::set(m, Point{1, 2}, "c"_gs);
    CHECK_EQ(gp::len(m), INT64_C(2));
    CHECK(gp::get(m, Point{1, 2}) == "c"_gs);
    CHECK(gp::get(m, Point{}) == ""_gs);
    CHECK(gp::hash_of(Point{1, 2}) != gp::hash_of(Point{2, 1}));

    gp::ptr<int64_t> p = gp::new_<int64_t>();
    gp::map<Mixed, int64_t> mx = gp::make_map<Mixed, int64_t>();
    gp::set(mx, Mixed{"k"_gs, -0.0, p, true}, 1);
    gp::set(mx, Mixed{gp::string::gp_copy("k"), 0.0, p, true}, 2); // equal: ±0, same bytes
    gp::set(mx, Mixed{"k"_gs, nan, p, true}, 3);                   // a NaN field: never equal
    gp::set(mx, Mixed{"k"_gs, nan, p, true}, 4);
    gp::set(mx, Mixed{"k"_gs, 0.0, gp::new_<int64_t>(), true}, 5); // another pointer
    CHECK_EQ(gp::len(mx), INT64_C(4));
    CHECK_EQ(gp::get(mx, Mixed{"k"_gs, 0.0, p, true}), INT64_C(2));
    CHECK_EQ(gp::get(mx, Mixed{"k"_gs, nan, p, true}), INT64_C(0));
    gp::erase(mx, Mixed{"k"_gs, nan, p, true});
    CHECK_EQ(gp::len(mx), INT64_C(4));
    CHECK_EQ(count(mx), INT64_C(4));
    // The stored key was replaced: its f is +0 now.
    int64_t pos_zero = 0;
    for (auto it = gp::range_map(mx); gp::range_next(it);) {
        pos_zero += it.gp_key().f == 0 && !std::signbit(it.gp_key().f) && it.gp_key().p == p;
    }
    CHECK_EQ(pos_zero, INT64_C(1));

    gp::map<Empty, int64_t> e = gp::make_map<Empty, int64_t>();
    gp::set(e, Empty{}, 1);
    gp::set(e, Empty{}, 2);
    CHECK_EQ(gp::len(e), INT64_C(1));

    gp::map<Arr3, int64_t> a = gp::make_map<Arr3, int64_t>();
    gp::set(a, Arr3{{1, 2, 3}}, 1);
    gp::set(a, Arr3{{3, 2, 1}}, 2);
    gp::set(a, Arr3{{1, 2, -0.0}}, 3);
    gp::set(a, Arr3{{1, 2, 0.0}}, 4);
    gp::set(a, Arr3{{nan, 2, 0.0}}, 5);
    gp::set(a, Arr3{{nan, 2, 0.0}}, 6);
    CHECK_EQ(gp::len(a), INT64_C(5));
    CHECK_EQ(gp::get(a, Arr3{{1, 2, 3}}), INT64_C(1));
    CHECK_EQ(gp::get(a, Arr3{{1, 2, 0}}), INT64_C(4));
    CHECK(gp::hash_of(Arr3{{1, 2, 3}}) != gp::hash_of(Arr3{{3, 2, 1}}));

    // [0]int: every key is the same key. [2]string, and [2][2]int nested.
    gp::map<gp::array<int64_t, 0>, int64_t> z = gp::make_map<gp::array<int64_t, 0>, int64_t>();
    gp::set(z, gp::array<int64_t, 0>{}, 1);
    gp::ref(z, gp::array<int64_t, 0>{}) += 5;
    CHECK_EQ(gp::len(z), INT64_C(1));
    CHECK_EQ(gp::get(z, gp::array<int64_t, 0>{}), INT64_C(6));
    using strs2 = gp::array<gp::string, 2>;
    gp::map<strs2, int64_t> sa = gp::make_map<strs2, int64_t>();
    gp::set(sa, strs2{{"a"_gs, "b"_gs}}, 1);
    gp::set(sa, strs2{{gp::string::gp_copy("a"), "b"_gs}}, 2);
    gp::set(sa, strs2{{"b"_gs, "a"_gs}}, 3);
    CHECK_EQ(gp::len(sa), INT64_C(2));
    CHECK_EQ(gp::get(sa, strs2{{"a"_gs, "b"_gs}}), INT64_C(2));
    using row = gp::array<int64_t, 2>;
    using grid = gp::array<row, 2>;
    gp::map<grid, int64_t> ga = gp::make_map<grid, int64_t>();
    gp::set(ga, grid{{row{1, 2}, row{3, 4}}}, 1);
    gp::set(ga, grid{{row{1, 2}, row{4, 3}}}, 2);
    CHECK_EQ(gp::len(ga), INT64_C(2));
    CHECK_EQ(gp::get(ga, grid{{row{1, 2}, row{3, 4}}}), INT64_C(1));
    static_assert(has_gp_hash<gp::array<int64_t, 2>> && has_gp_hash<gp::array<int64_t, 0>>);
    static_assert(!has_gp_hash<gp::array<gp::slice<int64_t>, 2>> &&
                      !has_gp_hash<gp::array<gp::slice<int64_t>, 0>>,
                  "an array of slices isn't comparable, so it has no hash");

    gp::map<NotComplex, int64_t> nc = gp::make_map<NotComplex, int64_t>();
    gp::set(nc, NotComplex{1, 2, 3}, 1);
    gp::set(nc, NotComplex{1, 2, 4}, 2);
    CHECK_EQ(gp::len(nc), INT64_C(2));
    CHECK(gp::hash_of(NotComplex{1, 2, 3}) == NotComplex{1, 2, 3}.gp_hash());

    gp::map<Outer, int64_t> o = gp::make_map<Outer, int64_t>();
    gp::set(o, Outer{{1, 2}, Arr3{{1, 1, 1}}}, 1);
    gp::set(o, Outer{{1, 2}, Arr3{{1, 1, -0.0}}}, 2);
    gp::set(o, Outer{{1, 2}, Arr3{{1, 1, 0.0}}}, 3);
    CHECK_EQ(gp::len(o), INT64_C(2));
    CHECK_EQ(gp::get(o, Outer{{1, 2}, Arr3{{1, 1, -0.0}}}), INT64_C(3));
}

void test_range_value() {
    // range_value reads the current entry's value after code that ran since range_next: the new
    // value of a store, the zero value after a delete or a clear (also for a NaN key).
    gp::map<gp::string, gp::string> m = gp::make_map<gp::string, gp::string>();
    gp::set(m, "k"_gs, "v"_gs);
    auto it = gp::range_map(m);
    CHECK(gp::range_next(it));
    CHECK(gp::range_value(it) == "v"_gs);
    gp::set(m, "k"_gs, "new"_gs);
    CHECK(gp::range_value(it) == "new"_gs);
    gp::erase(m, "k"_gs);
    CHECK(gp::range_value(it) == ""_gs);
    gp::set(m, "k"_gs, "again"_gs);
    CHECK(gp::range_value(it) == "again"_gs);
    CHECK(!gp::range_next(it));

    double zero = 0.0;
    gp::map<double, int64_t> f = gp::make_map<double, int64_t>();
    gp::set(f, zero / zero, 7);
    gp::set(f, 1.0, 8);
    gp::erase(f, 1.0);
    auto fit = gp::range_map(f);
    CHECK(gp::range_next(fit));
    CHECK_EQ(gp::range_value(fit), INT64_C(7));
    gp::clear(f);
    CHECK_EQ(gp::range_value(fit), INT64_C(0));
}

void test_range() {
    // Every entry once, with its value.
    imap m = gp::make_map<int64_t, int64_t>();
    for (int64_t i = 0; i < 100; i++) {
        gp::set(m, i, i * 10);
    }
    int64_t n = 0, sum = 0;
    for (auto it = gp::range_map(m); gp::range_next(it);) {
        n++;
        sum += it.gp_value() - it.gp_key() * 10;
    }
    CHECK_EQ(n, INT64_C(100));
    CHECK_EQ(sum, INT64_C(0));

    // Deleting entries that are not reached yet: they are not produced. Here the first
    // iteration deletes all the others.
    n = 0;
    for (auto it = gp::range_map(m); gp::range_next(it);) {
        int64_t k = it.gp_key();
        for (int64_t i = 0; i < 100; i++) {
            if (i != k) {
                gp::erase(m, i);
            }
        }
        n++;
    }
    CHECK_EQ(n, INT64_C(1));
    CHECK_EQ(gp::len(m), INT64_C(1));

    // Deleting the current entry, and each one as it is reached.
    for (int64_t i = 0; i < 100; i++) {
        gp::set(m, i, i);
    }
    n = 0;
    for (auto it = gp::range_map(m); gp::range_next(it);) {
        gp::erase(m, it.gp_key());
        n++;
    }
    CHECK_EQ(n, INT64_C(100));
    CHECK_EQ(gp::len(m), INT64_C(0));

    // Inserting during a range: the table grows (and rehashes) under the range; every old
    // entry is still produced once, the new ones are not (Go: may or may not be).
    for (int64_t i = 0; i < 10; i++) {
        gp::set(m, i, i);
    }
    n = 0;
    sum = 0;
    for (auto it = gp::range_map(m); gp::range_next(it);) {
        if (n == 0) {
            for (int64_t i = 1000; i < 3000; i++) {
                gp::set(m, i, 1);
            }
        }
        CHECK(it.gp_key() < 10);
        sum += it.gp_key();
        n++;
    }
    CHECK_EQ(n, INT64_C(10));
    CHECK_EQ(sum, INT64_C(45));
    CHECK_EQ(gp::len(m), INT64_C(2010));

    // Inserting and deleting: the remaining old entries are produced with their current values.
    gp::clear(m);
    for (int64_t i = 0; i < 10; i++) {
        gp::set(m, i, 0);
    }
    n = 0;
    std::vector<int64_t> seen;
    for (auto it = gp::range_map(m); gp::range_next(it);) {
        if (n == 0) {
            for (int64_t i = 0; i < 10; i++) {
                if (i != it.gp_key()) {
                    gp::set(m, i, i * 7); // a new value, read when the entry is reached
                }
            }
            for (int64_t i = 100; i < 2000; i++) {
                gp::set(m, i, 1);
            }
            gp::erase(m, (it.gp_key() + 1) % 10); // not reached yet: not produced
        } else {
            CHECK_EQ(it.gp_value(), it.gp_key() * 7);
        }
        seen.push_back(it.gp_key());
        n++;
    }
    CHECK_EQ(n, INT64_C(9));
    std::sort(seen.begin(), seen.end());
    CHECK(std::adjacent_find(seen.begin(), seen.end()) == seen.end());

    // A key deleted and stored again: produced at most once (Go: may or may not be).
    gp::clear(m);
    for (int64_t i = 0; i < 50; i++) {
        gp::set(m, i, i);
    }
    n = 0;
    for (auto it = gp::range_map(m); gp::range_next(it);) {
        if (n == 0) {
            for (int64_t i = 0; i < 50; i++) {
                gp::erase(m, i);
                gp::set(m, i, -i);
            }
        } else {
            CHECK_EQ(it.gp_value(), -it.gp_key());
        }
        n++;
    }
    CHECK_EQ(n, INT64_C(50));

    // clear during a range: nothing more is produced, NaN keys included.
    fmap f = gp::make_map<double, int64_t>();
    for (int64_t i = 0; i < 20; i++) {
        gp::set(f, i % 2 == 0 ? nan : static_cast<double>(i), i);
    }
    n = 0;
    for (auto it = gp::range_map(f); gp::range_next(it);) {
        gp::clear(f);
        n++;
    }
    CHECK_EQ(n, INT64_C(1));
    CHECK_EQ(gp::len(f), INT64_C(0));
    // ... and entries stored after the clear are not produced either.
    for (int64_t i = 0; i < 20; i++) {
        gp::set(f, i % 2 == 0 ? nan : static_cast<double>(i), i);
    }
    n = 0;
    for (auto it = gp::range_map(f); gp::range_next(it);) {
        if (n == 0) {
            gp::clear(f);
            for (int64_t i = 0; i < 20; i++) {
                gp::set(f, i % 2 == 0 ? nan : static_cast<double>(i), i);
            }
        }
        n++;
    }
    CHECK(n >= 1 && n <= 11); // the non-NaN keys may be found again; the old NaNs are gone

    // The range evaluates the map once: reassigning the variable doesn't change it, and the
    // old table lives until the range ends.
    imap r = gp::map_of<int64_t, int64_t>({{1, 1}, {2, 2}, {3, 3}});
    n = 0;
    sum = 0;
    for (auto it = gp::range_map(r); gp::range_next(it);) {
        r = nullptr;
        sum += it.gp_value();
        n++;
    }
    CHECK_EQ(n, INT64_C(3));
    CHECK_EQ(sum, INT64_C(6));

    // Nested ranges over one map, and a break (the range state is dropped early).
    imap q = gp::map_of<int64_t, int64_t>({{1, 1}, {2, 2}, {3, 3}});
    n = 0;
    for (auto it = gp::range_map(q); gp::range_next(it);) {
        for (auto in = gp::range_map(q); gp::range_next(in);) {
            n += it.gp_key() * in.gp_key();
        }
    }
    CHECK_EQ(n, INT64_C(36));
    n = 0;
    for (auto it = gp::range_map(q); gp::range_next(it);) {
        if (++n == 2) {
            break;
        }
    }
    CHECK_EQ(n, INT64_C(2));

    // The order is not randomized (D8): two ranges over an unchanged map agree.
    std::vector<int64_t> o1, o2;
    for (auto it = gp::range_map(m); gp::range_next(it);) {
        o1.push_back(it.gp_key());
    }
    for (auto it = gp::range_map(m); gp::range_next(it);) {
        o2.push_back(it.gp_key());
    }
    CHECK(o1 == o2);
}

void test_nested() {
    using inner = gp::map<gp::string, int64_t>;
    gp::map<gp::string, inner> mm = gp::make_map<gp::string, inner>();
    // mm["x"]["y"] = 1 with no mm["x"]: the inner map is nil.
    CHECK_EQ(panic_text([&] { gp::set(gp::get(mm, "x"_gs), "y"_gs, 1); }), nil_map_panic);
    CHECK_EQ(gp::len(mm), INT64_C(0));
    gp::set(mm, "x"_gs, gp::make_map<gp::string, int64_t>());
    gp::set(gp::get(mm, "x"_gs), "y"_gs, 1);
    gp::ref(gp::get(mm, "x"_gs), "y"_gs) += 5;
    // mm["z"]["w"]++: mm["z"] is read (nil), and the write to it panics; mm is unchanged.
    CHECK_EQ(panic_text([&] { gp::ref(gp::get(mm, "z"_gs), "w"_gs)++; }), nil_map_panic);
    CHECK_EQ(gp::len(mm), INT64_C(1));
    CHECK_EQ(gp::get(gp::get(mm, "x"_gs), "y"_gs), INT64_C(6));
    CHECK_EQ(gp::get(gp::get(mm, "nope"_gs), "y"_gs), INT64_C(0));
    CHECK_EQ(gp::len(gp::get(mm, "nope"_gs)), INT64_C(0));
    // The inner map is a reference: a copy taken out sees later writes.
    inner x = gp::get(mm, "x"_gs);
    gp::set(gp::get(mm, "x"_gs), "q"_gs, 2);
    CHECK_EQ(gp::len(x), INT64_C(2));
    gp::erase(mm, "x"_gs);
    CHECK_EQ(gp::get(x, "q"_gs), INT64_C(2)); // the inner map lives on through x

    // A map of slices.
    gp::map<int64_t, gp::slice<int64_t>> ms = gp::make_map<int64_t, gp::slice<int64_t>>();
    gp::set(ms, 1, gp::append(gp::get(ms, 1), 5));
    gp::set(ms, 1, gp::append(gp::get(ms, 1), 6));
    CHECK_EQ(gp::len(gp::get(ms, 1)), INT64_C(2));
    CHECK(gp::get(ms, 2) == nullptr);
}

int64_t live = 0;
int64_t bad_destroys = 0;
int64_t freed = 0;

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
    bool operator==(const Tracked& o) const { return v == o.v; }
    std::size_t gp_hash() const { return gp::hash_fields(v); }
};

void test_lifetimes() {
    {
        gp::map<Tracked, Tracked> m = gp::make_map<Tracked, Tracked>();
        for (int64_t i = 0; i < 50; i++) {
            gp::set(m, Tracked(i), Tracked(i));
        }
        CHECK_EQ(live, INT64_C(100));
        gp::set(m, Tracked(3), Tracked(30)); // overwrite: no new entry
        gp::ref(m, Tracked(4)).v = 40;
        gp::ref(m, Tracked(1000)).v = 1;
        CHECK_EQ(live, INT64_C(102));
        gp::erase(m, Tracked(5));
        CHECK_EQ(live, INT64_C(100));
        Tracked got = gp::get(m, Tracked(3));
        CHECK_EQ(got.v, INT64_C(30));
        CHECK_EQ(std::get<0>(gp::lookup(m, Tracked(4))).v, INT64_C(40));
        CHECK_EQ(live, INT64_C(101));
        {
            // A range copies the keys, and frees the copies with its state.
            int64_t n = 0;
            for (auto it = gp::range_map(m); gp::range_next(it);) {
                n++;
                gp::erase(m, Tracked(it.gp_key().v + 1));
            }
            CHECK(n > 0 && n <= 50);
        }
        gp::map<Tracked, Tracked> lit =
            gp::map_of<Tracked, Tracked>({{Tracked(1), Tracked(2)}, {Tracked(1), Tracked(3)}});
        CHECK_EQ(gp::get(lit, Tracked(1)).v, INT64_C(3));
        gp::map<Tracked, Tracked> copy = m;
        m = nullptr;
        gp::clear(copy);
        CHECK_EQ(live, INT64_C(3)); // got, and lit's key and value
        gp::set(copy, Tracked(1), Tracked(1));
        gp::map<int64_t, gp::map<Tracked, Tracked>> outer =
            gp::make_map<int64_t, gp::map<Tracked, Tracked>>();
        gp::set(outer, 1, copy);
        copy = nullptr;
    }
    CHECK_EQ(live, INT64_C(0));
    CHECK_EQ(bad_destroys, INT64_C(0));
}

// Chains: a list whose nodes hold maps that hold the next node, through pointers and by value.
struct PLink {
    gp::map<int64_t, gp::ptr<PLink>> next;
    ~PLink() { freed++; }
};

struct VLink {
    gp::map<int64_t, VLink> next;
    int64_t pad = 0;
    ~VLink() { freed++; }
};

gp::ptr<PLink> plist(int64_t n) {
    gp::ptr<PLink> head;
    for (int64_t i = 0; i < n; i++) {
        gp::ptr<PLink> l = gp::new_<PLink>();
        l->next = gp::make_map<int64_t, gp::ptr<PLink>>();
        gp::set(l->next, 0, head);
        head = std::move(l);
    }
    return head;
}

// Each chain is built and freed before the next one is built (memory).
void test_chains() {
    const int64_t n = chain_nodes;
    freed = 0;
    {
        gp::ptr<PLink> head = plist(n);
    } // dropped at the end of the scope
    CHECK_EQ(freed, n);
    CHECK(nothing_pending());

    freed = 0;
    gp::ptr<PLink> h = plist(n);
    h = nullptr; // dropped by an assignment
    CHECK_EQ(freed, n);
    CHECK(nothing_pending());

    // Values that hold maps that hold values: each VLink lives inside the previous one's map.
    freed = 0;
    gp::map<int64_t, VLink> top;
    for (int64_t i = 0; i < n; i++) {
        gp::map<int64_t, VLink> m = gp::make_map<int64_t, VLink>();
        gp::ref(m, 0).next = std::move(top);
        top = std::move(m);
    }
    int64_t made = freed;
    top = nullptr;
    CHECK_EQ(made, INT64_C(0));
    CHECK_EQ(freed, n);
    CHECK(nothing_pending());

    // Through erase and clear: the rest of the chain goes with the entry.
    freed = 0;
    gp::map<int64_t, gp::ptr<PLink>> holder = gp::make_map<int64_t, gp::ptr<PLink>>();
    gp::set(holder, 1, plist(n));
    gp::erase(holder, 1);
    CHECK_EQ(freed, n);
    gp::set(holder, 2, plist(n));
    gp::clear(holder);
    CHECK_EQ(freed, 2 * n);
    CHECK(nothing_pending());
}

// Stores whose source the store itself frees: m[k] = m[k].next, where the old value is the
// only owner of the new one.
struct SNode {
    gp::ptr<SNode> next;
    int64_t v = 0;
};

struct SVal {
    gp::ptr<SVal> child;
    gp::string s;
    int64_t v = 0;
};

void test_self_free() {
    gp::map<int64_t, gp::ptr<SNode>> m = gp::make_map<int64_t, gp::ptr<SNode>>();
    gp::ptr<SNode> a = gp::new_<SNode>(), b = gp::new_<SNode>();
    a->v = 1;
    b->v = 2;
    a->next = b;
    b = nullptr;
    gp::set(m, 1, std::move(a));
    gp::set(m, 1, gp::get(m, 1)->next); // frees a, the only owner of b, while b is stored
    CHECK_EQ(gp::get(m, 1)->v, INT64_C(2));
    gp::ptr<SNode> keep;
    gp::set(m, 2, gp::new_<SNode>());
    gp::get(m, 2)->next = gp::new_<SNode>();
    gp::get(m, 2)->next->v = 7;
    gp::ref(m, 2) = gp::get(m, 2)->next; // through ref: the right side is a copy first
    CHECK_EQ(gp::get(m, 2)->v, INT64_C(7));

    gp::map<int64_t, SVal> vm = gp::make_map<int64_t, SVal>();
    SVal inner{nullptr, gp::string::gp_copy("inner"), 5};
    SVal outer{gp::new_<SVal>(inner), gp::string::gp_copy("outer"), 1};
    inner = SVal{};
    gp::set(vm, 1, std::move(outer));
    gp::set(vm, 1, *gp::get(vm, 1).child); // the old value is the only owner of the child
    CHECK(gp::get(vm, 1).s == "inner"_gs && gp::get(vm, 1).v == 5);
}

void test_adl() {
    using gpkg_adl::K;
    gp::map<K, int64_t> m = gp::make_map<K, int64_t>();
    gp::set(m, K{1}, 1);
    gp::ref(m, K{2}) += 2;
    CHECK_EQ(gp::get(m, K{1}), INT64_C(1));
    CHECK(std::get<1>(gp::lookup(m, K{2})));
    for (auto it = gp::range_map(m); gp::range_next(it);) {
        gp::erase(m, K{2});
    }
    gp::erase(m, K{1});
    gp::clear(m);
    CHECK_EQ(gp::len(m), INT64_C(0));
    gp::map<K, int64_t> lit = gp::map_of<K, int64_t>({{K{1}, 1}});
    CHECK_EQ(gp::get(lit, K{1}), INT64_C(1));
    CHECK_EQ(gpkg_adl::hijacked, INT64_C(0));
}

// stress: a million inserts, lookups and deletes (stress_ops), for correctness under growth and
// shrinking and as a rough speed check (the times are in the job's report, not checked).
void test_stress() {
    const int64_t n = stress_ops;
    imap m = gp::make_map<int64_t, int64_t>();
    for (int64_t i = 0; i < n; i++) {
        gp::set(m, i * 7919 % n, i);
    }
    CHECK_EQ(gp::len(m), n);
    int64_t bad = 0;
    for (int64_t i = 0; i < n; i++) {
        bad += gp::get(m, i * 7919 % n) != i;
    }
    CHECK_EQ(bad, INT64_C(0));
    for (int64_t i = 0; i < n; i += 2) {
        gp::erase(m, i);
    }
    CHECK_EQ(gp::len(m), n / 2);
    int64_t odd = 0;
    for (auto it = gp::range_map(m); gp::range_next(it);) {
        odd += it.gp_key() % 2;
    }
    CHECK_EQ(odd, n / 2);
    for (int64_t i = 0; i < n; i++) {
        gp::ref(m, i)++;
    }
    CHECK_EQ(gp::len(m), n);
    for (int64_t i = 0; i < n; i++) {
        gp::erase(m, i);
    }
    CHECK_EQ(gp::len(m), INT64_C(0));

    // String keys: word counts.
    smap words = gp::make_map<gp::string, int64_t>();
    for (int64_t i = 0; i < n / 10; i++) {
        gp::ref(words, gp::string::gp_copy("w" + std::to_string(i % 5000))) += 1;
    }
    CHECK_EQ(gp::len(words), INT64_C(5000));
    CHECK_EQ(gp::get(words, "w42"_gs), n / 10 / 5000);
}

// bench: GOCPP_MAP_BENCH=1 prints the time of the stress loops to stderr (by hand only: the
// test fails on any stderr output).
void bench() {
    bool on = false;
    for (const gp::string& e : gp::envs()) {
        on = on || e == "GOCPP_MAP_BENCH=1"_gs;
    }
    if (!on) {
        return;
    }
    const int64_t n = 1'000'000;
    auto t0 = std::chrono::steady_clock::now();
    imap m = gp::make_map<int64_t, int64_t>();
    for (int64_t i = 0; i < n; i++) {
        gp::set(m, i * 7919 % n, i);
    }
    auto t1 = std::chrono::steady_clock::now();
    int64_t s = 0;
    for (int64_t i = 0; i < n; i++) {
        s += gp::get(m, i);
    }
    auto t2 = std::chrono::steady_clock::now();
    for (auto it = gp::range_map(m); gp::range_next(it);) {
        s += it.gp_value();
    }
    auto t3 = std::chrono::steady_clock::now();
    for (int64_t i = 0; i < n; i++) {
        gp::erase(m, i);
    }
    auto t4 = std::chrono::steady_clock::now();
    smap words = gp::make_map<gp::string, int64_t>();
    std::vector<gp::string> ws;
    for (int64_t i = 0; i < n; i++) {
        ws.push_back(gp::string::gp_copy("word" + std::to_string(i % 50000)));
    }
    auto t5 = std::chrono::steady_clock::now();
    for (const gp::string& w : ws) {
        gp::ref(words, w)++;
    }
    auto t6 = std::chrono::steady_clock::now();
    auto ms = [](auto a, auto b) {
        return std::to_string(std::chrono::duration<double, std::milli>(b - a).count());
    };
    std::string out = "insert " + ms(t0, t1) + " ms, get " + ms(t1, t2) + " ms, range " +
                      ms(t2, t3) + " ms, delete " + ms(t3, t4) + " ms, word counts " + ms(t5, t6) +
                      " ms (" + std::to_string(s % 10) + ")\n";
    gp::os::write(gp::os::stderr_fd, out.data(), out.size());
}

void run_tests() {
    test_nil();
    test_make();
    test_basic();
    test_literal();
    test_floats();
    test_keys();
    test_struct_keys();
    test_range();
    test_range_value();
    test_nested();
    test_lifetimes();
    test_chains();
    test_self_free();
    test_adl();
    test_stress();
    bench();
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
