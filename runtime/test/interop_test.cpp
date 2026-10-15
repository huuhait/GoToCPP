// The unit tests of the interop helpers (gocpp/interop.hpp, Phase 04 §2.5-§2.7 and §2.12):
// gp::deref, gp::range and gp::range_keys, gp::values, the implicit conversions at the C/C++
// boundary (gp::string to std::string_view, gp::slice to std::span, gp::ptr to T*), gp::cstr
// and cgo's helpers in gp::cgo. runtime/cxx_test.go compiles this file with the runtime's .cpp
// files, with g++ and clang++, at -O0, at -O2 and with ASan + UBSan (LeakSanitizer included),
// and runs it like test/unit.cpp: it prints "ok: N checks" and exits 0, or prints each failed
// check and exits 1. With an argument it runs one case that ends the program instead (the cases
// at the end of the file), which cxx_test.go checks from the outside: the expectations are what
// a cgo program printed with Go 1.25 (C.GoStringN(p, -1), C.GoBytes(nil, 3), ...).
#include <gocpp/rt.hpp>
#include <gocpp/os.hpp>

#include <algorithm>
#include <array>
#include <cmath>
#include <cstddef>
#include <cstdint>
#include <cstdio>
#include <cstring>
#include <deque>
#include <limits>
#include <list>
#include <map>
#include <memory>
#include <ranges>
#include <set>
#include <span>
#include <string>
#include <string_view>
#include <tuple>
#include <type_traits>
#include <unordered_map>
#include <utility>
#include <vector>

namespace {

int checks = 0;
int failures = 0;

std::string show(const std::string& v) {
    return '"' + v + '"';
}
std::string show(int64_t v) {
    return std::to_string(static_cast<long long>(v));
}
std::string show(int v) {
    return std::to_string(v);
}
std::string show(std::size_t v) {
    return std::to_string(static_cast<unsigned long long>(v));
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

std::string str(const gp::string& s) {
    return std::string(s.gp_view());
}

const std::string nil_msg = "runtime error: invalid memory address or nil pointer dereference";

// counted counts its copies, to show what is copied and what isn't.
struct counted {
    static inline int copies = 0;
    int v = 0;
    explicit counted(int x) : v(x) {}
    counted(const counted& o) : v(o.v) { copies++; }
    counted(counted&&) noexcept = default;
    counted& operator=(counted&&) noexcept = default;
    counted& operator=(const counted& o) {
        v = o.v;
        copies++;
        return *this;
    }
    int get() const { return v; }
};

struct S {
    int64_t a = 0;
    int64_t twice() const { return 2 * a; }
};

// Named types over the runtime types (gp::newtype, step 3.9), which derive from them.
struct NamedStr : gp::newtype<NamedStr, gp::string> {
    using gp::newtype<NamedStr, gp::string>::newtype;
};
struct NamedInts : gp::newtype<NamedInts, gp::slice<int64_t>> {
    using gp::newtype<NamedInts, gp::slice<int64_t>>::newtype;
};
struct NamedPtr : gp::newtype<NamedPtr, gp::ptr<S>> {
    using gp::newtype<NamedPtr, gp::ptr<S>>::newtype;
};

// --- gp::deref ---

// gp::cgo::as: Go's conversions of C/C++ values (int(x), C.int(d)), with Go's results where a
// static_cast would be undefined behavior (checked with Go 1.25 on amd64).
void test_as() {
    volatile double big = 1e300, nan = std::numeric_limits<double>::quiet_NaN();
    CHECK(gp::cgo::as<int64_t>(2.9) == 2);
    CHECK(gp::cgo::as<int64_t>(-2.9) == -2);
    CHECK(gp::cgo::as<int64_t>(double(big)) == INT64_MIN); // Go: int(1e300) is MinInt64
    CHECK(gp::cgo::as<int64_t>(double(nan)) == INT64_MIN);
    CHECK(gp::cgo::as<int>(double(big)) == INT32_MIN); // C.int: int32
    CHECK(gp::cgo::as<uint8_t>(300.0) == 44);          // through int32, low bits
    CHECK(gp::cgo::as<unsigned long long>(double(big)) == (uint64_t(1) << 63));
    CHECK(gp::cgo::as<long long>(2.5f) == 2);
    CHECK(gp::cgo::as<long long>(1e300L) == INT64_MIN);
    CHECK(std::isinf(gp::cgo::as<float>(double(big))) && gp::cgo::as<float>(-double(big)) < 0);
    CHECK(std::isnan(gp::cgo::as<float>(double(nan))));
    CHECK(gp::cgo::as<float>(0x1.fffffefp127) == std::numeric_limits<float>::max()); // rounds down
    CHECK(std::isinf(gp::cgo::as<float>(0x1.ffffffp127)));                           // half way: up
    CHECK(gp::cgo::as<float>(1.5) == 1.5f);
    CHECK(gp::cgo::as<double>(3) == 3.0);
    CHECK(gp::cgo::as<int>(int64_t(1) << 40) == 0); // integers: C++20 wraps, like Go
    CHECK(gp::cgo::as<bool>(true) == true); // only a bool converts to bool (a static_assert)
    // case_eq: a case of a switch on a C/C++ value, C++'s == for numbers, enums and pointers.
    enum mode { mode_a, mode_b };
    CHECK(gp::cgo::case_eq(-1, EOF));
    CHECK(gp::cgo::case_eq(mode_b, 1));
    CHECK(!gp::cgo::case_eq(static_cast<const char*>(nullptr), "x"));
}

void test_deref() {
    S s{21};
    S* p = &s;
    static_assert(std::is_same_v<decltype(gp::deref(p)), S&>, "a raw pointer: one level");
    gp::deref(p).a = 5;
    CHECK_EQ(s.a, int64_t(5));
    CHECK_EQ(gp::deref(p).twice(), int64_t(10));
    const S* cp = &s;
    static_assert(std::is_same_v<decltype(gp::deref(cp)), const S&>);
    CHECK(&gp::deref(cp) == &s);
    S** pp = &p;
    static_assert(std::is_same_v<decltype(gp::deref(pp)), S*&>, "only one level, like Go");
    CHECK(gp::deref(pp) == p);

    // A nil raw pointer panics like Go's nil dereference, before anything reads it.
    S* nilp = nullptr;
    CHECK_EQ(panic_text([&] { (void)gp::deref(nilp).a; }), nil_msg);

    // Values pass through as references, without a copy.
    static_assert(std::is_same_v<decltype(gp::deref(s)), S&>);
    CHECK(&gp::deref(s) == &s);
    const S cs{3};
    static_assert(std::is_same_v<decltype(gp::deref(cs)), const S&>);
    CHECK(&gp::deref(cs) == &cs);
    static_assert(std::is_same_v<decltype(gp::deref(S{})), S&&>, "a temporary stays an xvalue");
    counted::copies = 0;
    counted c(7);
    CHECK_EQ(gp::deref(c).get(), 7);
    CHECK_EQ(gp::deref(counted(8)).get(), 8);
    CHECK_EQ(counted::copies, 0);

    // A gocpp pointer that comes back as an opaque value (Go auto-dereferences *T), with its
    // nil panic: here directly, and below as an element of a C++ container.
    gp::ptr<S> gp = gp::new_<S>(S{4});
    static_assert(std::is_same_v<decltype(gp::deref(gp)), S&>);
    CHECK(&gp::deref(gp) == gp.gp_get());
    CHECK_EQ(gp::deref(gp).twice(), int64_t(8));
    gp::ptr<S> gnil;
    CHECK_EQ(panic_text([&] { (void)gp::deref(gnil).a; }), nil_msg);
    NamedPtr np(gp);
    static_assert(std::is_same_v<decltype(gp::deref(np)), S&>, "a named pointer type too");
    CHECK_EQ(gp::deref(np).a, int64_t(4));
    // std.vector[*S]: its elements are gocpp pointers, and so is what a C++ template returns.
    std::vector<gp::ptr<S>> elems{gp, gnil};
    static_assert(std::is_same_v<decltype(gp::deref(elems[0])), S&>);
    gp::deref(elems[0]).a = 12;
    CHECK_EQ(gp->a, int64_t(12));
    CHECK_EQ(gp::deref(elems.front()).twice(), int64_t(24));
    CHECK_EQ(gp::deref(std::max(gp, gp, [](const auto&, const auto&) { return false; })).a,
             int64_t(12));
    CHECK_EQ(panic_text([&] { (void)gp::deref(elems[1]).a; }), nil_msg);

    // C++ smart pointers and iterators keep their own members: (*sp).m is the explicit form.
    auto up = std::make_unique<S>(S{9});
    static_assert(std::is_same_v<decltype(gp::deref(up)), std::unique_ptr<S>&>);
    CHECK_EQ(gp::deref(up).get()->a, int64_t(9));
    gp::deref(up).reset();
    CHECK(up == nullptr);
    auto sp = std::make_shared<S>(S{1});
    static_assert(std::is_same_v<decltype(gp::deref(sp)), std::shared_ptr<S>&>);
    CHECK_EQ(int64_t(gp::deref(sp).use_count()), int64_t(1));
    void* vp = &s;
    static_assert(std::is_same_v<decltype(gp::deref(vp)), void*&>, "void* has no members");
}

// --- gp::range and gp::range_keys ---

std::vector<std::string> long_strings() {
    // Longer than any small-string buffer, so a dangling reference reads freed heap memory,
    // which ASan reports.
    return {std::string(40, 'a'), std::string(40, 'b'), std::string(40, 'c')};
}

// tracked is a container that says when it is destroyed: a temporary passed to gp::range must
// live until the loop ends.
struct tracked {
    std::vector<int> v{1, 2, 3};
    bool* alive;
    explicit tracked(bool* a) : alive(a) { *a = true; }
    tracked(tracked&& o) noexcept : v(std::move(o.v)), alive(o.alive) { o.alive = nullptr; }
    ~tracked() {
        if (alive != nullptr) {
            *alive = false;
        }
    }
    auto begin() { return v.begin(); }
    auto end() { return v.end(); }
};

void test_range_vector() {
    std::vector<int> v{10, 20, 30};
    std::string got;
    for (auto&& [i, x] : gp::range(v)) {
        static_assert(std::is_same_v<decltype(i), int64_t>, "the index is Go's int");
        static_assert(std::is_same_v<decltype(x), int>);
        got += std::to_string(i) + ":" + std::to_string(x) + " ";
        x = 0; // a copy, as in Go
    }
    CHECK_EQ(got, std::string("0:10 1:20 2:30 "));
    CHECK_EQ(v[0], 10);
    static_assert(std::is_same_v<decltype(*begin(std::declval<decltype(gp::range(v))&>())),
                                 std::pair<int64_t, int>>);

    // The one-variable form: only the index.
    got.clear();
    for (auto&& i : gp::range_keys(v)) {
        static_assert(std::is_same_v<decltype(i), int64_t&&>);
        got += std::to_string(i);
    }
    CHECK_EQ(got, std::string("012"));

    // The forms with a blank variable (and none at all) compile without warnings (-Werror).
    int64_t sum = 0;
    for ([[maybe_unused]] auto&& [gp_k, x] : gp::range(v)) {
        sum += x;
    }
    CHECK_EQ(sum, int64_t(60));
    int n = 0;
    for ([[maybe_unused]] auto&& gp_k : gp::range_keys(v)) {
        n++;
    }
    CHECK_EQ(n, 3);
    for ([[maybe_unused]] auto&& [gp_k, gp_v] : gp::range(v)) {
    }

    // The length is read once: a push_back that reallocates doesn't break the loop, and the
    // new elements aren't visited, as with a Go slice.
    std::vector<int> grow{1, 2, 3};
    grow.shrink_to_fit();
    const int* before = grow.data();
    n = 0;
    for (auto&& [i, x] : gp::range(grow)) {
        grow.push_back(x * 10);
        n++;
    }
    CHECK_EQ(n, 3);
    CHECK(grow.data() != before); // it did reallocate
    CHECK(grow == (std::vector<int>{1, 2, 3, 10, 20, 30}));
    std::deque<int> dq{1, 2};
    n = 0;
    for (auto&& [i, x] : gp::range(dq)) {
        dq.push_back(x);
        dq.push_front(0);
        n++;
    }
    CHECK_EQ(n, 2);
    CHECK_EQ(dq.size(), std::size_t(6));

    // A container that shrinks: the next index past its end panics like Go's index check,
    // instead of reading a destroyed element.
    std::vector<int> shrink{1, 2, 3, 4};
    std::string seen;
    CHECK_EQ(panic_text([&] {
                 for (auto&& [i, x] : gp::range(shrink)) {
                     seen += std::to_string(x);
                     shrink.pop_back();
                 }
             }),
             std::string("runtime error: index out of range [2] with length 2"));
    CHECK_EQ(seen, std::string("12"));
    // range_keys reads no element, so it doesn't check.
    n = 0;
    for ([[maybe_unused]] auto&& i : gp::range_keys(shrink)) {
        shrink.clear();
        n++;
    }
    CHECK_EQ(n, 2);

    // Elements that can't be copied work with range_keys.
    std::vector<std::unique_ptr<int>> owners;
    owners.push_back(std::make_unique<int>(5));
    for (auto&& i : gp::range_keys(owners)) {
        CHECK_EQ(*owners[static_cast<std::size_t>(i)], 5);
    }

    // Each element is copied exactly once per step, and range_keys copies none.
    std::vector<counted> cs;
    cs.emplace_back(1);
    cs.emplace_back(2);
    counted::copies = 0;
    int total = 0;
    for (auto&& [i, c] : gp::range(cs)) {
        total += c.get();
    }
    CHECK_EQ(total, 3);
    CHECK_EQ(counted::copies, 2);
    counted::copies = 0;
    for ([[maybe_unused]] auto&& i : gp::range_keys(cs)) {
    }
    CHECK_EQ(counted::copies, 0);

    // A const container, and std::vector<bool> (whose elements are proxies: v is a bool).
    const std::vector<int>& cv = v;
    sum = 0;
    for (auto&& [i, x] : gp::range(cv)) {
        sum += i * x;
    }
    CHECK_EQ(sum, int64_t(80));
    std::vector<bool> bits{true, false, true};
    got.clear();
    for (auto&& [i, b] : gp::range(bits)) {
        static_assert(std::is_same_v<decltype(b), bool>);
        got += b ? "1" : "0";
        b = false;
    }
    CHECK_EQ(got, std::string("101"));
    CHECK(bits[0]);
}

void test_range_temporary() {
    // A temporary container is held inside the view: under C++20 a reference to it would
    // dangle in the loop (ASan would report a heap-use-after-free here).
    std::string got;
    for (auto&& [i, s] : gp::range(long_strings())) {
        got += s.substr(0, 1);
    }
    CHECK_EQ(got, std::string("abc"));
    int64_t n = 0;
    for (auto&& i : gp::range_keys(long_strings())) {
        n += i;
    }
    CHECK_EQ(n, int64_t(3));
    bool alive = false;
    int steps = 0;
    for (auto&& [i, x] : gp::range(tracked(&alive))) {
        CHECK(alive);
        steps += x;
    }
    CHECK(!alive);
    CHECK_EQ(steps, 6);
    // A view as a temporary: walked by index (iota is random-access and sized).
    n = 0;
    for (auto&& [i, x] : gp::range(std::views::iota(5, 8))) {
        n += i * 10 + x;
    }
    CHECK_EQ(n, int64_t(0 + 5 + 10 + 6 + 20 + 7));
}

void test_range_other() {
    // C arrays, std::array, std::string, std::string_view and std::span.
    int64_t arr[3] = {7, 8, 9};
    int64_t sum = 0;
    for (auto&& [i, x] : gp::range(arr)) {
        sum += i * x;
    }
    CHECK_EQ(sum, int64_t(8 + 18));
    int n = 0;
    for ([[maybe_unused]] auto&& i : gp::range_keys(arr)) {
        n++;
    }
    CHECK_EQ(n, 3);
    // A C array of C arrays: range_keys and c[i] (gp::range is rejected with a clear message,
    // runtime/reject_test.go).
    int grid[2][3] = {{1, 2, 3}, {4, 5, 6}};
    sum = 0;
    for (auto&& i : gp::range_keys(grid)) {
        sum += grid[i][2];
    }
    CHECK_EQ(sum, int64_t(9));
    std::array<int, 2> sa{4, 5};
    sum = 0;
    for (auto&& [i, x] : gp::range(sa)) {
        sum += x;
    }
    CHECK_EQ(sum, int64_t(9));
    std::string got;
    for (auto&& [i, ch] : gp::range(std::string("hey"))) {
        static_assert(std::is_same_v<decltype(ch), char>);
        got += std::to_string(i) + ch;
    }
    CHECK_EQ(got, std::string("0h1e2y"));
    got.clear();
    for (auto&& [i, ch] : gp::range(std::string_view("ab"))) {
        got += ch;
    }
    CHECK_EQ(got, std::string("ab"));
    std::span<int64_t> sp(arr);
    sum = 0;
    for (auto&& [i, x] : gp::range(sp)) {
        sum += x;
    }
    CHECK_EQ(sum, int64_t(24));

    // Lists and sets: walked by iterator, with a counted index.
    std::list<std::string> names{"x", "y"};
    got.clear();
    for (auto&& [i, s] : gp::range(names)) {
        got += std::to_string(i) + s;
        s = "changed";
    }
    CHECK_EQ(got, std::string("0x1y"));
    CHECK_EQ(names.front(), std::string("x"));
    std::set<int> set{3, 1, 2};
    got.clear();
    for (auto&& [i, x] : gp::range(set)) {
        got += std::to_string(i) + std::to_string(x);
    }
    CHECK_EQ(got, std::string("011223"));
    got.clear();
    for (auto&& i : gp::range_keys(set)) {
        got += std::to_string(i);
    }
    CHECK_EQ(got, std::string("012"));

    // Views whose end is another type (a sentinel), or that aren't sized: by iterator.
    std::vector<int> v{1, 2, 3, 4, 5, 6};
    got.clear();
    for (auto&& [i, x] : gp::range(std::views::take_while(v, [](int x) { return x < 4; }))) {
        got += std::to_string(i) + std::to_string(x);
    }
    CHECK_EQ(got, std::string("011223"));
    got.clear();
    for (auto&& [i, x] : gp::range(std::views::filter(v, [](int x) { return x % 2 == 0; }))) {
        got += std::to_string(i) + std::to_string(x);
    }
    CHECK_EQ(got, std::string("021426"));
}

void test_range_maps() {
    std::map<std::string, int> ages{{"bob", 31}, {"al", 40}};
    std::string got;
    for (auto&& [k, v] : gp::range(ages)) {
        static_assert(std::is_same_v<decltype(k), std::string>, "the key is a copy, not const");
        static_assert(std::is_same_v<decltype(v), int>);
        got += k + "=" + std::to_string(v) + " ";
        k = "x";
        v = 0;
    }
    CHECK_EQ(got, std::string("al=40 bob=31 "));
    CHECK_EQ(ages["bob"], 31);
    CHECK_EQ(ages.size(), std::size_t(2));
    got.clear();
    for (auto&& k : gp::range_keys(ages)) {
        static_assert(std::is_same_v<decltype(k), std::string&&>);
        got += k + " ";
    }
    CHECK_EQ(got, std::string("al bob "));
    std::unordered_map<int, int> um{{1, 10}, {2, 20}, {3, 30}};
    int64_t ks = 0, vs = 0;
    for (auto&& [k, v] : gp::range(um)) {
        ks += k;
        vs += v;
    }
    CHECK_EQ(ks, int64_t(6));
    CHECK_EQ(vs, int64_t(60));
    std::multimap<int, std::string> mm{{1, "a"}, {1, "b"}, {0, "c"}};
    got.clear();
    for (auto&& [k, v] : gp::range(mm)) {
        got += std::to_string(k) + v;
    }
    CHECK_EQ(got, std::string("0c1a1b"));
    // A temporary map.
    got.clear();
    for (auto&& [k, v] : gp::range(
             std::map<std::string, std::string>{{std::string(40, 'k'), std::string(40, 'v')}})) {
        got += k.substr(0, 1) + v.substr(0, 1);
    }
    CHECK_EQ(got, std::string("kv"));
}

// --- gp::values ---

void test_values() {
    static_assert(std::is_same_v<decltype(std::minmax(1, 2)), std::pair<const int&, const int&>>);
    static_assert(std::is_same_v<decltype(gp::values(std::minmax(1, 2))), std::pair<int, int>>);
    auto [lo, hi] = gp::values(std::minmax(3, 1));
    CHECK_EQ(lo, 1);
    CHECK_EQ(hi, 3);
    lo = 7; // Go's variables can be assigned
    CHECK_EQ(lo, 7);

    // Temporaries with heap memory: without gp::values a and b would refer to destroyed
    // strings (ASan: stack-use-after-scope or heap-use-after-free; the "dangle" case below
    // shows that ASan does report it).
    auto [a, b] = gp::values(std::minmax(std::string(40, 'y'), std::string(40, 'x')));
    CHECK_EQ(a, std::string(40, 'x'));
    CHECK_EQ(b, std::string(40, 'y'));

    // With variables, the results are copies, not aliases.
    int x = 5, y = 9;
    auto [p, q] = gp::values(std::minmax(x, y));
    p = 100;
    CHECK_EQ(x, 5);
    x = 0;
    CHECK_EQ(q, 9);
    CHECK_EQ(p, 100);
    int64_t i = 1;
    std::string s = "s";
    auto [ci, cs] = gp::values(std::tie(i, s));
    static_assert(
        std::is_same_v<decltype(gp::values(std::tie(i, s))), std::tuple<int64_t, std::string>>);
    ci = 2;
    cs = "t";
    CHECK_EQ(i, int64_t(1));
    CHECK_EQ(s, std::string("s"));

    // Value members are moved from a temporary, const is dropped, and other values are copies.
    static_assert(std::is_same_v<decltype(gp::values(std::pair<const int, std::string>(1, "a"))),
                                 std::pair<int, std::string>>);
    static_assert(std::is_same_v<decltype(gp::values(std::tuple<const int&, std::string&&, double>(
                                     1, std::string(), 2.0))),
                                 std::tuple<int, std::string, double>>);
    counted::copies = 0;
    auto [m1, m2] =
        gp::values(std::pair<std::unique_ptr<int>, counted>(std::make_unique<int>(4), counted(1)));
    CHECK_EQ(*m1, 4);
    CHECK_EQ(m2.get(), 1);
    CHECK_EQ(counted::copies, 0); // moved, never copied
    static_assert(std::is_same_v<decltype(gp::values(5)), int>);
    // An rvalue-reference member is copied too: what it refers to stays as it is.
    std::string src(40, 's');
    auto [fwd] = gp::values(std::forward_as_tuple(std::move(src)));
    static_assert(std::is_same_v<decltype(gp::values(std::forward_as_tuple(std::move(src)))),
                                 std::tuple<std::string>>);
    CHECK_EQ(fwd, std::string(40, 's'));
    CHECK_EQ(src, std::string(40, 's'));
    // A named pair of values is copied, not moved.
    std::pair<std::string, int> named(std::string(40, 'n'), 1);
    auto [n1, n2] = gp::values(named);
    CHECK_EQ(n1, named.first);
    CHECK_EQ(n2, 1);
    S sv{3};
    auto copy = gp::values(sv);
    copy.a = 4;
    CHECK_EQ(sv.a, int64_t(3));
    CHECK_EQ(copy.a, int64_t(4));
}

// --- the conversions at the boundary ---

std::size_t view_size(std::string_view v) {
    return v.size();
}

// An overload set that takes std::string or const char*: neither is implicit for a gocpp
// string (only std::string_view is, §2.6), so a call doesn't compile.
int by_string(const std::string&) {
    return 1;
}
int by_string(const char*) {
    return 2;
}
template <typename T>
concept calls_by_string = requires(const T& s) { by_string(s); };

// std::string + std::string_view: the view is chosen (no ambiguity).
int str_or_view(const std::string&) {
    return 1;
}
int str_or_view(std::string_view) {
    return 2;
}
// A gocpp string overload wins over the view.
int gs_or_view(const gp::string&) {
    return 1;
}
int gs_or_view(std::string_view) {
    return 2;
}

void test_string_view() {
    static_assert(std::is_convertible_v<gp::string, std::string_view>);
    static_assert(std::is_convertible_v<NamedStr, std::string_view>);
    static_assert(!std::is_convertible_v<gp::string, std::string>);
    static_assert(!std::is_convertible_v<gp::string, const char*>);
    static_assert(std::is_constructible_v<std::string, gp::string>, "std.string(s) is explicit");
    static_assert(!calls_by_string<gp::string>);
    static_assert(calls_by_string<std::string>);

    gp::string s = gp::concat("ab"_gs, "c\0d"_gs);
    std::string_view v = s;
    CHECK(v.data() == s.gp_data()); // no copy
    CHECK_EQ(v.size(), std::size_t(5));
    CHECK_EQ(view_size(s), std::size_t(5));
    CHECK_EQ(view_size(NamedStr("xy"_gs)), std::size_t(2));
    CHECK_EQ(view_size(gp::string()), std::size_t(0));
    CHECK_EQ(std::string(s), std::string("abc\0d", 5));
    CHECK_EQ(str_or_view(s), 2);
    CHECK_EQ(gs_or_view(s), 1);
    // The overload sets themselves, with C++ arguments.
    CHECK_EQ(by_string(std::string("x")), 1);
    CHECK_EQ(by_string("x"), 2);
    CHECK_EQ(str_or_view(std::string("x")), 1);
    CHECK_EQ(gs_or_view(std::string_view("x")), 2);
    std::string cpp = "x";
    cpp += s; // std::string's operator+= takes anything that converts to std::string_view
    CHECK_EQ(cpp.size(), std::size_t(6));
    CHECK(std::string_view("abc") == gp::sub(s, 0, 3)); // std::string_view's ==
    CHECK(gp::sub(s, 0, 3) == std::string_view("abc"));

    // gocpp's own operations are the same as before: they match gp::string exactly.
    CHECK("abc"_gs == gp::sub(s, 0, 3));
    CHECK("abd"_gs > gp::sub(s, 0, 3));
    CHECK_EQ(str("a"_gs + "b"_gs), std::string("ab"));
    gp::map<gp::string, int64_t> m = gp::make_map<gp::string, int64_t>();
    gp::set(m, "k"_gs, 1);
    CHECK_EQ(gp::get(m, gp::concat("k"_gs, ""_gs)), int64_t(1));
    CHECK_EQ(str(gp::fmtshim::Sprint(s, "x"_gs)), std::string("abc\0dx", 6));
    CHECK_EQ(str(gp::fmtshim::Sprintf("%q"_gs, "a"_gs)), std::string("\"a\""));
}

int64_t span_sum(std::span<const int64_t> s) {
    int64_t n = 0;
    for (int64_t x : s) {
        n += x;
    }
    return n;
}
void span_double(std::span<int64_t> s) {
    for (int64_t& x : s) {
        x *= 2;
    }
}

void test_span() {
    using ints = gp::slice<int64_t>;
    static_assert(std::is_convertible_v<ints, std::span<int64_t>>);
    static_assert(std::is_convertible_v<ints, std::span<const int64_t>>);
    static_assert(std::is_convertible_v<NamedInts, std::span<int64_t>>);
    static_assert(!std::is_convertible_v<ints, std::span<int32_t>>);
    static_assert(!std::is_convertible_v<ints, std::span<int64_t, 3>>, "dynamic extent only");
    static_assert(!std::is_convertible_v<ints, bool>);

    ints s = gp::slice_of<int64_t>({1, 2, 3, 4});
    std::span<int64_t> sp = s;
    CHECK(sp.data() == s.gp_data());
    CHECK_EQ(sp.size(), std::size_t(4));
    CHECK_EQ(span_sum(s), int64_t(10));
    span_double(s); // writes are seen through the slice
    CHECK_EQ(s[3], int64_t(8));
    ints part = gp::sub(s, 1, 2); // len 1, cap 3: the span has len elements
    CHECK_EQ(std::span<int64_t>(part).size(), std::size_t(1));
    CHECK_EQ(span_sum(part), int64_t(4));
    CHECK_EQ(span_sum(NamedInts(s)), int64_t(20));
    ints nil;
    std::span<int64_t> empty = nil;
    CHECK(empty.data() == nullptr);
    CHECK(empty.empty());
    CHECK_EQ(span_sum(gp::make_slice<int64_t>(0)), int64_t(0));
    CHECK(s != nullptr); // == nil is the same as before
    CHECK_EQ(gp::len(s), int64_t(4));
}

int64_t raw_get(const S* p) {
    return p == nullptr ? -1 : p->a;
}
void raw_set(S* p, int64_t v) {
    p->a = v;
}
const void* raw_void(const void* p) {
    return p;
}
// f(bool) + f(void*): the pointer wins for a raw pointer; a gocpp pointer is ambiguous there
// (the deleted conversion to bool), so it doesn't compile.
int bool_or_void(bool) {
    return 1;
}
int bool_or_void(const void*) {
    return 2;
}
template <typename T>
concept calls_bool_or_void = requires(const T& p) { bool_or_void(p); };
// f(S*) + f(const gp::ptr<S>&): the gocpp pointer matches its own overload exactly.
int raw_or_ptr(S*) {
    return 1;
}
int raw_or_ptr(const gp::ptr<S>&) {
    return 2;
}
int takes_bool(bool b) {
    return b ? 1 : 0;
}
template <typename T>
concept calls_takes_bool = requires(const T& p) { takes_bool(p); };
template <typename T>
concept has_not = requires(const T& p) { !p; };
template <typename T>
concept is_condition = requires(const T& p) { p ? 1 : 2; };
template <typename T>
concept has_and = requires(const T& p) { p && p; };
// The raw pointer's other operations: none compiles for a gocpp pointer (the deleted
// conversions; `delete p` is in runtime/reject_test.go, as g++ rejects it with a hard error
// even inside a requires-expression).
template <typename T>
concept has_index = requires(const T& p) { p[0]; };
template <typename T>
concept has_less = requires(const T& p) { p < p; };
template <typename T>
concept has_order = requires(const T& p) { p <=> p; };
template <typename T>
concept has_arith = requires(const T& p) { p + 1; } || requires(const T& p) { p - p; } ||
                    requires(const T& p) { +p; };

void test_ptr() {
    using P = gp::ptr<S>;
    static_assert(std::is_convertible_v<P, S*>);
    static_assert(std::is_convertible_v<P, const S*>);
    static_assert(std::is_convertible_v<P, void*>);
    static_assert(std::is_convertible_v<P, const void*>);
    static_assert(std::is_convertible_v<gp::ptr<void>, void*>);
    static_assert(std::is_convertible_v<NamedPtr, S*>);
    // Not to bool (Go has no pointer conditions), and not to other pointer types.
    static_assert(!std::is_convertible_v<P, bool>);
    static_assert(!std::is_constructible_v<bool, P>);
    static_assert(!has_not<P> && !is_condition<P> && !has_and<P>);
    static_assert(has_not<S*> && is_condition<S*> && has_and<S*>);
    static_assert(!has_index<P> && !has_less<P> && !has_order<P> && !has_arith<P>);
    static_assert(!has_index<gp::ptr<gp::array<int64_t, 3>>>, "p[i] would index past the box");
    static_assert(!has_index<NamedPtr> && !has_less<NamedPtr>);
    static_assert(has_index<S*> && has_less<S*> && has_order<S*> && has_arith<S*>);
    // What takes a gocpp pointer still does: T*, const T*, void*, const void* parameters (only
    // T* converts to them), a C++ container of gocpp pointers, an overload set with both.
    std::vector<P> ps;
    ps.push_back(gp::new_<S>());
    CHECK_EQ(ps.size(), std::size_t(1));
    CHECK_EQ(raw_or_ptr(ps[0]), 2);
    CHECK_EQ(raw_or_ptr(ps[0].gp_get()), 1);
    static_assert(!calls_takes_bool<P>);
    static_assert(!calls_bool_or_void<P>);
    static_assert(calls_bool_or_void<S*>);
    static_assert(!std::is_convertible_v<P, int64_t*>);
    static_assert(!std::is_convertible_v<P, gp::ptr<int64_t>>);
    static_assert(!std::is_convertible_v<P, int64_t>);
    static_assert(!std::equality_comparable_with<gp::ptr<S>, gp::ptr<int64_t>>);

    P p = gp::new_<S>(S{6});
    S* raw = p;
    CHECK(raw == p.gp_get());
    CHECK_EQ(raw_get(p), int64_t(6));
    raw_set(p, 7);
    CHECK_EQ(p->a, int64_t(7));
    CHECK(raw_void(p) == p.gp_get());
    CHECK_EQ(bool_or_void(raw), 2);
    CHECK_EQ(bool_or_void(p.gp_get()), 2);
    CHECK_EQ(bool_or_void(true), 1);
    CHECK_EQ(takes_bool(p != nullptr), 1); // a nil test is written as one
    CHECK_EQ(raw_get(NamedPtr(p)), int64_t(7));
    P nil;
    CHECK(static_cast<S*>(nil) == nullptr);
    CHECK_EQ(raw_get(nil), int64_t(-1));
    gp::ptr<void> up = gp::ptr<void>(p, p.gp_get());
    CHECK(static_cast<void*>(up) == p.gp_get());

    // The operators of gp::ptr are the same as before: == compares gocpp pointers, == nullptr
    // is a nil test, and *p and p->f keep the nil panic (not a raw dereference).
    P q = p;
    CHECK(p == q);
    CHECK(!(p != q));
    CHECK(p != nullptr);
    CHECK(nullptr != p);
    CHECK(nil == nullptr);
    CHECK(gp::new_<S>() != p);
    CHECK_EQ(panic_text([&] { (void)(*nil).a; }), nil_msg);
    CHECK_EQ(panic_text([&] { (void)nil->a; }), nil_msg);

    // As map keys: by address, through std::hash<gp::ptr> and gp::map.
    gp::map<P, int64_t> m = gp::make_map<P, int64_t>();
    gp::set(m, p, 1);
    CHECK_EQ(gp::get(m, q), int64_t(1));
    CHECK_EQ(gp::get(m, gp::new_<S>()), int64_t(0));
    std::unordered_map<P, int> um;
    um[p] = 2;
    CHECK_EQ(um[q], 2);
    CHECK_EQ(um.size(), std::size_t(1));

    // fmt prints a gocpp pointer as before: &{...} at the top level, an address inside.
    CHECK_EQ(str(gp::fmtshim::Sprint(gp::new_<int64_t>(1))).substr(0, 2), std::string("0x"));
    gp::ptr<int64_t> ip = gp::new_<int64_t>(5);
    char buf[32];
    std::snprintf(buf, sizeof buf, "0x%llx",
                  static_cast<unsigned long long>(reinterpret_cast<std::uintptr_t>(ip.gp_get())));
    CHECK_EQ(str(gp::fmtshim::Sprint(ip)), std::string(buf));
    CHECK_EQ(str(gp::fmtshim::Sprint(gp::ptr<int64_t>())), std::string("<nil>"));
}

// --- gp::cstr ---

void test_cstr() {
    // A literal is NUL-terminated already: no copy.
    gp::string lit = "hello"_gs;
    CHECK(gp::cstr(lit).gp_ptr() == lit.gp_data());
    CHECK(std::strcmp(gp::cstr(lit), "hello") == 0);
    // A suffix of a literal ends at its NUL: no copy. A prefix doesn't: a copy.
    gp::string tail = gp::sub(lit, 3);
    CHECK(gp::cstr(tail).gp_ptr() == tail.gp_data());
    gp::string head = gp::sub(lit, 0, 2);
    CHECK(gp::cstr(head).gp_ptr() != head.gp_data());
    CHECK(std::strcmp(gp::cstr(head), "he") == 0);
    // string(r) of an ASCII rune is static and NUL-terminated (also the last one, 0x7f).
    gp::string a = gp::rune_to_string(int32_t('a'));
    CHECK(gp::cstr(a).gp_ptr() == a.gp_data());
    CHECK(std::strcmp(gp::cstr(a), "a") == 0);
    gp::string del = gp::rune_to_string(int32_t(0x7f));
    CHECK(gp::cstr(del).gp_ptr() == del.gp_data());
    CHECK_EQ(std::strlen(gp::cstr(del)), std::size_t(1));
    // A string made at run time: a copy.
    gp::string made = gp::concat("ab"_gs, "cd"_gs);
    CHECK(gp::cstr(made).gp_ptr() != made.gp_data());
    CHECK(std::strcmp(gp::cstr(made), "abcd") == 0);
    // Empty strings give "", never null.
    CHECK(std::strcmp(gp::cstr(gp::string()), "") == 0);
    CHECK(std::strcmp(gp::cstr(""_gs), "") == 0);
    CHECK(gp::cstr(gp::sub(made, 1, 1)).gp_ptr() != nullptr);
    // A NUL inside the bytes ends the C string early, as with C.CString.
    CHECK_EQ(std::strlen(gp::cstr(gp::concat("a\0b"_gs, "c"_gs))), std::size_t(1));
    // A named string type.
    CHECK(std::strcmp(gp::cstr(NamedStr(made)), "abcd") == 0);

    // A holder in a variable lives as long as the variable, also after the string is gone,
    // and copies and moves stay valid (short strings: inside std::string's own buffer).
    for (std::size_t len : {std::size_t(3), std::size_t(100)}) {
        gp::string s = gp::string::gp_copy(std::string(len, 'z'));
        auto h = gp::cstr(s);
        s = gp::string(); // drops the only owner of the bytes
        auto h2 = h;
        auto h3 = std::move(h2);
        CHECK_EQ(std::strlen(h), len);
        CHECK_EQ(std::strlen(h3), len);
        CHECK(h.gp_ptr() != h3.gp_ptr());
    }

    // C++ string literals and char arrays (an untyped constant is written as a literal).
    static const char word[] = "word";
    CHECK(gp::cstr(word).gp_ptr() == word);
    CHECK(std::strcmp(gp::cstr("lit"), "lit") == 0);
    char full[4] = {'a', 'b', 'c', 'd'}; // no NUL: copied with one
    CHECK(gp::cstr(full).gp_ptr() != full);
    CHECK(std::strcmp(gp::cstr(full), "abcd") == 0);
    char name[8] = "ab";
    CHECK(gp::cstr(name).gp_ptr() == name);

    // A variadic C function needs the explicit form.
    char out[16];
    std::snprintf(out, sizeof out, "[%s]", gp::cstr(made).gp_ptr());
    CHECK_EQ(std::string(out), std::string("[abcd]"));
    static_assert(std::is_convertible_v<gp::cstr_holder, const char*>);
    static_assert(!std::is_convertible_v<gp::cstr_holder, char*>);
}

// --- cgo's helpers ---

void test_cgo() {
    char* p = gp::cgo::CString("hello"_gs);
    CHECK(std::strcmp(p, "hello") == 0);
    gp::cgo::free(p); // ASan checks that it was malloc'ed
    p = gp::cgo::CString(""_gs);
    CHECK(p != nullptr);
    CHECK_EQ(p[0], '\0');
    gp::cgo::free(p);
    p = gp::cgo::CString(gp::concat("a\0b"_gs, ""_gs, "c"_gs));
    CHECK(std::memcmp(p, "a\0bc\0", 5) == 0);
    gp::cgo::free(p);
    p = gp::cgo::CString(NamedStr("n"_gs));
    CHECK(std::strcmp(p, "n") == 0);
    gp::cgo::free(p);
    gp::cgo::free(nullptr);

    // GoString: any byte-sized char pointer; null gives "".
    CHECK_EQ(str(gp::cgo::GoString(nullptr)), std::string());
    CHECK_EQ(str(gp::cgo::GoString(static_cast<const char*>(nullptr))), std::string());
    char buf[] = "hello";
    gp::string g = gp::cgo::GoString(buf);
    buf[0] = 'j'; // a copy
    CHECK_EQ(str(g), std::string("hello"));
    const unsigned char ubuf[] = "uhi";
    CHECK_EQ(str(gp::cgo::GoString(ubuf)), std::string("uhi"));
    const signed char sbuf[] = "s";
    CHECK_EQ(str(gp::cgo::GoString(sbuf)), std::string("s"));
    CHECK_EQ(str(gp::cgo::GoString(u8"u8")), std::string("u8"));
    std::byte bb[2] = {std::byte{'b'}, std::byte{0}};
    CHECK_EQ(str(gp::cgo::GoString(bb)), std::string("b"));
    CHECK_EQ(str(gp::cgo::GoString(static_cast<const char*>(""))), std::string());

    // GoStringN: n bytes, NULs too; 0 doesn't read p.
    CHECK_EQ(str(gp::cgo::GoStringN(buf, 3)), std::string("jel"));
    CHECK_EQ(str(gp::cgo::GoStringN("a\0b", 3)), std::string("a\0b", 3));
    CHECK_EQ(str(gp::cgo::GoStringN(nullptr, 0)), std::string());
    CHECK_EQ(str(gp::cgo::GoStringN(static_cast<char*>(nullptr), int64_t(0))), std::string());
    CHECK_EQ(str(gp::cgo::GoStringN(ubuf, uint8_t(2))), std::string("uh"));
    CHECK_EQ(panic_text([] { (void)gp::cgo::GoStringN(static_cast<char*>(nullptr), 3); }), nil_msg);
    CHECK_EQ(panic_text([] { (void)gp::cgo::GoStringN(nullptr, 1); }), nil_msg);

    // GoBytes: a new []byte; 0 gives an empty slice that is not nil.
    gp::slice<uint8_t> b = gp::cgo::GoBytes(buf, 6);
    CHECK_EQ(gp::len(b), int64_t(6));
    CHECK_EQ(gp::cap(b), int64_t(6));
    CHECK_EQ(int(b[0]), int('j'));
    CHECK_EQ(int(b[5]), 0);
    buf[1] = 'x';
    CHECK_EQ(int(b[1]), int('e')); // a copy
    gp::slice<uint8_t> zero = gp::cgo::GoBytes(buf, 0);
    CHECK(zero != nullptr);
    CHECK_EQ(gp::len(zero), int64_t(0));
    CHECK(gp::cgo::GoBytes(nullptr, 0) != nullptr);
    CHECK_EQ(gp::len(gp::cgo::GoBytes(nullptr, 0)), int64_t(0));
    const std::string gobytes_msg = "runtime error: gobytes: length out of range";
    CHECK_EQ(panic_text([&] { (void)gp::cgo::GoBytes(buf, -1); }), gobytes_msg);
    CHECK_EQ(panic_text([&] { (void)gp::cgo::GoBytes(buf, ~uint64_t(0)); }), gobytes_msg);
    CHECK_EQ(panic_text([&] { (void)gp::cgo::GoBytes(buf, int64_t(1) << 49); }), gobytes_msg);
    CHECK_EQ(panic_text([] { (void)gp::cgo::GoBytes(nullptr, 3); }), nil_msg);
    // unsafe.Pointer is a gp::ptr<void>, and any gocpp pointer converts to a raw one.
    gp::ptr<int64_t> ip = gp::new_<int64_t>(0x0102030405060708);
    gp::slice<uint8_t> ib = gp::cgo::GoBytes(ip, 8);
    int64_t back = 0;
    std::memcpy(&back, ib.gp_data(), 8);
    CHECK_EQ(back, int64_t(0x0102030405060708));
    gp::ptr<void> vp = gp::ptr<void>(ip, ip.gp_get());
    CHECK_EQ(gp::len(gp::cgo::GoBytes(vp, int32_t(2))), int64_t(2));
}

void run_tests() {
    test_as();
    test_deref();
    test_range_vector();
    test_range_temporary();
    test_range_other();
    test_range_maps();
    test_values();
    test_string_view();
    test_span();
    test_ptr();
    test_cstr();
    test_cgo();
    if (failures > 0) {
        std::string msg =
            "FAIL: " + std::to_string(failures) + " of " + std::to_string(checks) + " checks\n";
        gp::os::write(gp::os::stderr_fd, msg.data(), msg.size());
        gp::os::exit(1);
    }
    gp::fmtshim::Printf("ok: %d checks\n"_gs, int64_t(checks));
}

// The cases that end the program (cxx_test.go's runInterop checks their output and exit
// code), from the program's argument.
const char* the_case = nullptr;

char cbuf[] = "hello";

void run_case() {
    std::string c = the_case;
    if (c == "strn-neg") {
        // Go: "fatal error: out of memory", exit code 2.
        gp::println(gp::cgo::GoStringN(cbuf, -1));
    } else if (c == "strn-nil") {
        // Go: the nil dereference panic, with the signal line.
        gp::println(gp::cgo::GoStringN(static_cast<char*>(nullptr), 3));
    } else if (c == "bytes-neg") {
        gp::println(gp::len(gp::cgo::GoBytes(cbuf, -1)));
    } else if (c == "bytes-nil") {
        gp::println(gp::len(gp::cgo::GoBytes(nullptr, 3)));
    } else if (c == "deref-nil") {
        S* p = nullptr;
        gp::println(gp::deref(p).a);
    } else if (c == "dangle") {
        // Without gp::values: the references dangle, which ASan must report (only run with
        // ASan; without it this reads freed memory).
#if defined(__GNUC__) && !defined(__clang__)
#pragma GCC diagnostic push
#pragma GCC diagnostic ignored "-Wdangling-reference"
#endif
        auto [a, b] = std::minmax(std::string(40, 'y'), std::string(40, 'x'));
#if defined(__GNUC__) && !defined(__clang__)
#pragma GCC diagnostic pop
#endif
        gp::println(gp::string::gp_copy(a), gp::string::gp_copy(b));
    } else {
        gp::println("unknown case"_gs);
    }
}

void no_init() {}

} // namespace

int main(int argc, char** argv) {
    if (argc == 2) {
        the_case = argv[1];
        return gp::run_main(argc, argv, &no_init, &run_case);
    }
    return gp::run_main(argc, argv, &no_init, &run_tests);
}
