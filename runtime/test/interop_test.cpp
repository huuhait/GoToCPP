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
#include <functional>
#include <limits>
#include <list>
#include <map>
#include <memory>
#include <optional>
#include <ranges>
#include <set>
#include <stdexcept>
#include <span>
#include <string>
#include <string_view>
#include <thread>
#include <tuple>
#include <type_traits>
#include <unordered_map>
#include <utility>
#include <vector>

#include "threads_ended.hpp"

namespace {

int cb_sink = 0;

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
    struct Bits {
        unsigned ready : 1;
        unsigned mode : 3;
        int sign : 5;
    };
    Bits bits{1, 5, -3};
    CHECK(gp::cgo::as<uint64_t>(bits.mode) == 5); // a bit-field can't bind to a reference
    CHECK(gp::cgo::as<int64_t>(bits.sign) == -3 && gp::cgo::as<bool>(bool(bits.ready)));
    // case_eq: a case of a switch on a C/C++ value, C++'s == for numbers, enums and pointers.
    enum mode { mode_a, mode_b };
    CHECK(gp::cgo::case_eq(-1, EOF));
    CHECK(gp::cgo::case_eq(mode_b, 1));
    const char* text = "x";
    // (a string literal as the case would compare addresses: rejected)
    CHECK(!gp::cgo::case_eq(static_cast<const char*>(nullptr), text));
}

// gp::nonnull and gp::smartnn: the nil checks of typed interop's p->m and sp->m (ADR-087). The
// panic must come from the check itself (a C++ panic here, not a signal).
void test_nonnull() {
    S s{1};
    S* p = &s;
    S* nilp = nullptr;
    CHECK(gp::nonnull(p) == p);
    CHECK_EQ(gp::nonnull(p)->a, int64_t(1));
    CHECK_EQ(panic_text([&] { (void)gp::nonnull(nilp)->a; }), nil_msg);
    gp::ptr<S> g = gp::new_<S>(S{2});
    CHECK_EQ(gp::nonnull(g)->a, int64_t(2));
    gp::ptr<S> gnil;
    CHECK_EQ(panic_text([&] { (void)gp::nonnull(gnil)->a; }), nil_msg);

    std::unique_ptr<S> up;
    CHECK_EQ(panic_text([&] { (void)gp::smartnn(up)->a; }), nil_msg);
    up = std::make_unique<S>(S{3});
    CHECK_EQ(gp::smartnn(up)->a, int64_t(3));
    std::shared_ptr<S> sp;
    CHECK_EQ(panic_text([&] { (void)gp::smartnn(sp)->a; }), nil_msg);
    sp = std::make_shared<S>(S{4});
    CHECK_EQ(gp::smartnn(sp)->a, int64_t(4));
    std::optional<S> opt;
    CHECK_EQ(panic_text([&] { (void)gp::smartnn(opt)->a; }), nil_msg);
    opt = S{5};
    CHECK_EQ(gp::smartnn(opt)->a, int64_t(5));
    // An iterator is passed through: no check, and no copy.
    std::vector<S> v{S{6}};
    CHECK_EQ(gp::smartnn(v.begin())->a, int64_t(6));
    static_assert(std::is_same_v<decltype(gp::smartnn(up)), std::unique_ptr<S>&>);
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

// gp::cgo::star: *x of a C/C++ value. Raw and gocpp pointers get Go's nil check (gp::deref), the
// rest keeps C++'s operator* (smart pointers, iterators). gp::cgo::cond: a bool condition.
void test_star_cond() {
    S s{6};
    S* p = &s;
    static_assert(std::is_same_v<decltype(gp::cgo::star(p)), S&>);
    gp::cgo::star(p).a = 7;
    CHECK_EQ(s.a, int64_t(7));
    S* nilp = nullptr;
    CHECK_EQ(panic_text([&] { (void)gp::cgo::star(nilp).a; }), nil_msg);
    gp::ptr<S> gp = gp::new_<S>(S{3});
    static_assert(std::is_same_v<decltype(gp::cgo::star(gp)), S&>);
    CHECK_EQ(gp::cgo::star(gp).a, int64_t(3));
    auto up = std::make_unique<S>(S{9});
    static_assert(std::is_same_v<decltype(gp::cgo::star(up)), S&>, "a smart pointer's own *");
    CHECK_EQ(gp::cgo::star(up).twice(), int64_t(18));
    std::vector<int> v{4, 5};
    auto it = v.begin();
    gp::cgo::star(it) = 40;
    CHECK_EQ(v[0], 40);
    CHECK_EQ(gp::cgo::star(v.begin() + 1), 5);
    CHECK(gp::cgo::cond(v.size() == 2));
    CHECK(!gp::cgo::cond(v.empty()));
    std::vector<bool> bs{true, false};
    CHECK(gp::cgo::cond(bs[0]) && !gp::cgo::cond(bs[1])); // the proxy converts to bool
    // to_bool: a comparison of C/C++ values is a Go bool.
    static_assert(std::is_same_v<decltype(gp::cgo::to_bool(v.size() == 2)), bool>);
    CHECK(gp::cgo::to_bool(it == v.begin()) && !gp::cgo::to_bool(p == nullptr));
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
    for (auto&& x : gp::range_values(v)) {
        sum += x;
    }
    CHECK_EQ(sum, int64_t(60));
    int n = 0;
    for ([[maybe_unused]] auto&& gp_k : gp::range_skip(v)) {
        n++;
    }
    CHECK_EQ(n, 3);
    for ([[maybe_unused]] auto&& gp_k : gp::range_skip(v)) {
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
    // No discarded binding requires a copy, even if both key and element are move-only.
    std::map<std::unique_ptr<int>, std::unique_ptr<int>> owners;
    owners.emplace(std::make_unique<int>(3), std::make_unique<int>(7));
    int visited = 0;
    for ([[maybe_unused]] auto&& step : gp::range_skip(owners)) {
        ++visited;
    }
    CHECK_EQ(visited, 1);
    CHECK_EQ(*owners.begin()->first, 3);
    std::map<int, std::unique_ptr<int>> mapped;
    mapped.emplace(3, std::make_unique<int>(7));
    for (auto&& key : gp::range_keys(mapped)) {
        CHECK_EQ(key, 3);
    }
    std::map<std::unique_ptr<int>, int> keyed;
    keyed.emplace(std::make_unique<int>(3), 7);
    for (auto&& value : gp::range_values(keyed)) {
        CHECK_EQ(value, 7);
        value = 0;
    }
    CHECK_EQ(keyed.begin()->second, 7);
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

    // One variable from a C/C++ call, x := lib.f() (step 4.7): a result that is no pair or tuple
    // costs one move and no copy over auto x = f(), and a reference result is copied, as auto
    // copies it. A C array decays to a pointer, as with auto.
    struct moved {
        int* moves;
        explicit moved(int* m) : moves(m) {}
        moved(const moved& o) : moves(o.moves) { counted::copies++; }
        moved(moved&& o) noexcept : moves(o.moves) { ++*moves; }
    };
    int moves = 0;
    counted::copies = 0;
    auto mv = gp::values(moved(&moves));
    CHECK_EQ(moves, 1);
    CHECK_EQ(counted::copies, 0);
    moved named_mv(&moves);
    moved& ref_mv = named_mv;
    moves = 0;
    auto mv2 = gp::values(ref_mv);
    CHECK_EQ(moves, 0);
    CHECK_EQ(counted::copies, 1);
    (void)mv;
    (void)mv2;
    static int arr[3] = {1, 2, 3};
    static_assert(std::is_same_v<decltype(gp::values(arr)), int*>);
    static_assert(std::is_same_v<decltype(gp::values(std::string("s"))), std::string>);
    auto p1 = gp::values(std::minmax(std::string(40, 'q'), std::string(40, 'p')));
    CHECK_EQ(p1.first, std::string(40, 'p'));
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
    CHECK_EQ(str(gp::fmt::Sprint(s, "x"_gs)), std::string("abc\0dx", 6));
    CHECK_EQ(str(gp::fmt::Sprintf("%q"_gs, "a"_gs)), std::string("\"a\""));

    // A gocpp string that comes through C++ compares with the C literal of a string constant
    // (nodes[0].Name == "a", Phase 04 step 4.9): the literal's bytes before its NUL, also a NUL
    // inside it; a char array without a NUL at its end is all its bytes.
    CHECK(gp::sub(s, 0, 3) == "abc");
    CHECK("abc" == gp::sub(s, 0, 3));
    CHECK(s == "abc\0d");
    CHECK(s != "abc");
    CHECK(gp::string() == "");
    CHECK(gp::sub(s, 0, 3) < "abd" && "abd" > gp::sub(s, 0, 3) && gp::sub(s, 0, 3) >= "ab");
    CHECK("\xff"_gs > "a"); // as unsigned bytes, like Go
    const char arr[2] = {'a', 'b'};
    CHECK("ab"_gs == arr);
    CHECK(NamedStr("x"_gs) == "x");

    // A C++ string literal converts implicitly to a gocpp string, a copy (step 4.9): for C++
    // functions that take one (v.push_back("x") of a std.vector[string]).
    std::vector<gp::string> names;
    names.push_back("x");
    names.emplace_back("y\0z");
    std::vector<gp::string> lits{"a", "b"};
    CHECK(names[0] == "x"_gs && gp::len(names[1]) == 3 && lits[1] == "b"_gs);
    std::map<gp::string, int64_t> ages;
    ages["bob"] = 31;
    CHECK(ages.count("bob") == 1 && ages.find("eve") == ages.end());
    gp::string lit = "lit";
    CHECK(lit == "lit"_gs && !lit.gp_c_terminated()); // a copy, with an owner
    char local[4] = {'a', 'b', 'c', 'd'};             // no NUL at its end: all its bytes
    gp::string copied = local;
    local[0] = 'z';
    CHECK(copied == "abcd"_gs);
    static_assert(!std::is_convertible_v<const char*, gp::string>, "a char pointer has no length");

    // string(x) of a C++ string: a copy of its bytes (gp::cgo::as<gp::string>).
    std::string cs("x\0y", 3);
    gp::string c = gp::cgo::as<gp::string>(cs);
    cs[0] = 'z';
    CHECK(c == "x\0y"_gs);
    CHECK(gp::cgo::as<gp::string>(std::string_view("view")) == "view"_gs);
    CHECK(gp::len(gp::cgo::as<gp::string>(std::string())) == 0);
    CHECK(gp::cgo::as<gp::string>(s).gp_data() == s.gp_data()); // a gocpp string: no copy
    CHECK(gp::cgo::as<gp::string>(65) == "A"_gs);               // string(r) of a C integer
    CHECK(gp::cgo::as<gp::string>('a') == "a"_gs);
    CHECK(gp::cgo::as<gp::string>(-1) == "\uFFFD"_gs);
    enum letter { letter_b = 'b' };
    CHECK(gp::cgo::as<gp::string>(letter_b) == "b"_gs);
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
    CHECK_EQ(
        str(gp::fmt::Sprint(gp::fmt::as(gp::td_ptr<int64_t>(&gp::td_int), gp::new_<int64_t>(1))))
            .substr(0, 2),
        std::string("0x"));
    gp::ptr<int64_t> ip = gp::new_<int64_t>(5);
    char buf[32];
    std::snprintf(buf, sizeof buf, "0x%llx",
                  static_cast<unsigned long long>(reinterpret_cast<std::uintptr_t>(ip.gp_get())));
    CHECK_EQ(str(gp::fmt::Sprint(gp::fmt::as(gp::td_ptr<int64_t>(&gp::td_int), ip))),
             std::string(buf));
    CHECK_EQ(
        str(gp::fmt::Sprint(gp::fmt::as(gp::td_ptr<int64_t>(&gp::td_int), gp::ptr<int64_t>()))),
        std::string("<nil>"));
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

// --- C.wstr, C.GoStringW, C.GoStringWN (step 14.4, ADR-082) ---

template <typename W>
std::basic_string<W> wide_of(const char* utf8) {
    return gp::detail::to_wide<W>(utf8, static_cast<int64_t>(std::strlen(utf8)));
}

void test_wstr() {
    // UTF-16 units (char16_t), whatever the size of wchar_t.
    CHECK(wide_of<char16_t>("abc") == u"abc");
    CHECK(wide_of<char16_t>("h\xC3\xA9llo") == u"h\u00e9llo");     // 2 bytes
    CHECK(wide_of<char16_t>("\xE4\xB8\x96") == u"\u4e16");         // 3 bytes
    CHECK(wide_of<char16_t>("\xF0\x9F\x99\x82") == u"\U0001F642"); // a pair
    CHECK_EQ(int(wide_of<char16_t>("\xF0\x9F\x99\x82").size()), 2);
    // The low bits of the pair: U+1F643 and U+10FFFF (the largest) round trip too.
    CHECK(wide_of<char16_t>("\xF0\x9F\x99\x83") == u"\U0001F643");
    CHECK(wide_of<char16_t>("\xF4\x8F\xBF\xBF") ==
          (std::u16string() + char16_t(0xDBFF) + char16_t(0xDFFF)));
    CHECK_EQ(str(gp::detail::from_wide(u"\U0001F643\U0010FFFF", 4)),
             std::string("\xF0\x9F\x99\x83\xF4\x8F\xBF\xBF"));
    // An invalid byte is U+FFFD (Go's UTF16FromString), also a truncated sequence and an overlong
    // one.
    CHECK(wide_of<char16_t>("a\xFFz") == u"a\ufffdz");
    CHECK(wide_of<char16_t>("\xE4\xB8") == u"\ufffd\ufffd");
    CHECK(wide_of<char16_t>("\xC0\x80") == u"\ufffd\ufffd");
    // A WTF-8 surrogate is one lone unit: ED A0 80 (D800), ED BF BF (DFFF).
    CHECK(wide_of<char16_t>("x\xED\xA0\x80y") == (std::u16string(u"x") + char16_t(0xD800) + u"y"));
    CHECK(wide_of<char16_t>("\xED\xBF\xBF") == std::u16string(1, char16_t(0xDFFF)));
    // A valid pair written as two WTF-8 surrogates stays two lone units (Go does not join them).
    CHECK_EQ(int(wide_of<char16_t>("\xED\xA0\xBD\xED\xB9\x82").size()), 2);
    // ED 9F BF is U+D7FF, a real character, not a surrogate.
    CHECK(wide_of<char16_t>("\xED\x9F\xBF") == std::u16string(1, char16_t(0xD7FF)));
    // UTF-32 units: a pair is one unit, a surrogate stays a lone value.
    CHECK(wide_of<char32_t>("\xF0\x9F\x99\x82") == U"\U0001F642");
    CHECK(wide_of<char32_t>("\xED\xA0\x80") == std::u32string(1, char32_t(0xD800)));
    CHECK(wide_of<char32_t>("a\xFF") == U"a\ufffd");

    // Back to a gocpp string.
    CHECK_EQ(str(gp::detail::from_wide(u"h\u00e9llo \u4e16 \U0001F642", 10)),
             std::string("h\xC3\xA9llo \xE4\xB8\x96 \xF0\x9F\x99\x82"));
    const char16_t lone[] = {
        u'x', char16_t(0xD800), u'y', char16_t(0xDFFF), char16_t(0xD800), char16_t(0xDC00)};
    // lone high, lone low, then a real pair (U+10000).
    CHECK_EQ(str(gp::detail::from_wide(lone, 6)),
             std::string("x\xED\xA0\x80y\xED\xBF\xBF\xF0\x90\x80\x80"));
    const char16_t tail[] = {u'a', char16_t(0xD83D)};
    CHECK_EQ(str(gp::detail::from_wide(tail, 2)),
             std::string("a\xED\xA0\xBD")); // a high surrogate at the end
    const char32_t wide32[] = {U'a', char32_t(0x1F642), char32_t(0xD800), char32_t(0x110000),
                               char32_t(0xFFFFFFFF)};
    CHECK_EQ(str(gp::detail::from_wide(wide32, 5)),
             std::string("a\xF0\x9F\x99\x82\xED\xA0\x80\xEF\xBF\xBD\xEF\xBF\xBD"));
    // A round trip of ill-formed text is lossless (WTF-8), unless a high and a low surrogate follow
    // each other: they join into one character, in Go too.
    const std::string ill = "k\xED\xA0\x80z\xED\xB0\x80";
    auto w16 = wide_of<char16_t>(ill.c_str());
    CHECK_EQ(str(gp::detail::from_wide(w16.data(), int64_t(w16.size()))), ill);

    // C.wstr(s): a holder of wchar_t units, NUL-terminated; a NUL inside ends the C string.
    gp::wstr_holder h = gp::wstr("h\xC3\xA9llo"_gs);
    CHECK(h.gp_ptr()[0] == L'h' && h.gp_ptr()[1] == wchar_t(0xE9) && h.gp_ptr()[5] == 0);
    // The implicit conversion to const wchar_t*: valid to the end of the statement.
    CHECK_EQ(int(gp::detail::wide_length<wchar_t>(gp::wstr("abc"_gs))), 3);
    CHECK_EQ(int(gp::detail::wide_length(gp::wstr("").gp_ptr())), 0);
    CHECK_EQ(int(gp::detail::wide_length(gp::wstr(gp::concat("a\0b"_gs, ""_gs, "c"_gs)).gp_ptr())),
             1);
    CHECK_EQ(int(gp::detail::wide_length(gp::wstr("lit").gp_ptr())), 3);
    gp::wstr_holder copy = h; // a copy keeps its own units
    CHECK(copy.gp_ptr() != h.gp_ptr() && copy.gp_ptr()[1] == wchar_t(0xE9));
    if constexpr (sizeof(wchar_t) == 2) {
        CHECK_EQ(int(gp::detail::wide_length(gp::wstr("\xF0\x9F\x99\x82"_gs).gp_ptr())), 2);
    } else {
        CHECK_EQ(int(gp::detail::wide_length(gp::wstr("\xF0\x9F\x99\x82"_gs).gp_ptr())), 1);
    }

    // WString: a malloc'ed copy, for a pointer that C keeps; C.free frees it (ASan checks).
    wchar_t* kept = gp::cgo::WString("k\xC3\xA9\xF0\x9F\x99\x82"_gs);
    CHECK_EQ(str(gp::cgo::GoStringW(kept)), std::string("k\xC3\xA9\xF0\x9F\x99\x82"));
    CHECK_EQ(int(gp::detail::wide_length(kept)), sizeof(wchar_t) == 2 ? 4 : 3);
    gp::cgo::free(kept);
    wchar_t* empty = gp::cgo::WString(""_gs);
    CHECK(empty != nullptr && empty[0] == 0);
    gp::cgo::free(empty);

    // GoStringW: any pointer to 2- or 4-byte integers; null gives "".
    CHECK_EQ(str(gp::cgo::GoStringW(nullptr)), std::string());
    CHECK_EQ(str(gp::cgo::GoStringW(static_cast<const wchar_t*>(nullptr))), std::string());
    CHECK_EQ(str(gp::cgo::GoStringW(gp::wstr("h\xC3\xA9llo \xE4\xB8\x96"_gs).gp_ptr())),
             std::string("h\xC3\xA9llo \xE4\xB8\x96"));
    CHECK_EQ(str(gp::cgo::GoStringW(L"wide")), std::string("wide"));
    CHECK_EQ(str(gp::cgo::GoStringW(u"u16 \u00e9")), std::string("u16 \xC3\xA9"));
    CHECK_EQ(str(gp::cgo::GoStringW(U"u32 \U0001F642")), std::string("u32 \xF0\x9F\x99\x82"));
    uint16_t u16buf[] = {'o', 'k', 0, 'x'};
    CHECK_EQ(str(gp::cgo::GoStringW(u16buf)), std::string("ok")); // up to the NUL
    gp::ptr<uint16_t> gpbuf = gp::new_<uint16_t>(uint16_t('p'));  // one unit: no NUL after it
    CHECK_EQ(str(gp::cgo::GoStringWN(gpbuf, 1)), std::string("p"));
    gp::ptr<uint16_t> gpzero = gp::new_<uint16_t>(uint16_t(0));
    CHECK_EQ(str(gp::cgo::GoStringW(gpzero)), std::string());
    // GoStringWN: n units, NULs too; 0 doesn't read p.
    CHECK_EQ(str(gp::cgo::GoStringWN(u16buf, 4)), std::string("ok\0x", 4));
    CHECK_EQ(str(gp::cgo::GoStringWN(u16buf, 1)), std::string("o"));
    CHECK_EQ(str(gp::cgo::GoStringWN(nullptr, 0)), std::string());
    CHECK_EQ(str(gp::cgo::GoStringWN(static_cast<const wchar_t*>(nullptr), int64_t(0))),
             std::string());
    CHECK_EQ(panic_text([] { (void)gp::cgo::GoStringWN(static_cast<const wchar_t*>(nullptr), 3); }),
             nil_msg);
    CHECK_EQ(panic_text([] { (void)gp::cgo::GoStringWN(nullptr, 1); }), nil_msg);
}

// --- boxes and result slots (step 4.8) ---

// Counts what happens to it, and can't be default-constructed or assigned.
struct Tracked {
    static inline int made = 0, copies = 0, moves = 0, dtors = 0;
    int id;
    explicit Tracked(int i) : id(i) { made++; }
    Tracked(const Tracked& o) : id(o.id) { copies++; }
    Tracked(Tracked&& o) noexcept : id(o.id) {
        moves++;
        o.id = -1;
    }
    ~Tracked() { dtors++; }
    static void reset() { made = copies = moves = dtors = 0; }
};

// Can't be copied or moved at all (a std::mutex, an atomic): only built in place.
struct Pinned {
    static inline int dtors = 0;
    int v;
    explicit Pinned(int x) : v(x) {}
    Pinned(const Pinned&) = delete;
    Pinned& operator=(const Pinned&) = delete;
    ~Pinned() { dtors++; }
};

// Move-only, default-constructible and move-assignable (std::unique_ptr).
using UP = std::unique_ptr<int>;

Pinned make_pinned(int x) {
    return Pinned(x);
}

struct NoAssign { // default-constructible, but not assignable
    const int c = 7;
    int d = 0;
};

void test_box() {
    // box(make): a movable object is built in the box, not moved into it.
    Tracked::reset();
    {
        auto b = gp::cgo::box([&] { return Tracked(5); });
        static_assert(std::is_same_v<decltype(b), gp::ptr<Tracked>>);
        CHECK_EQ((*b).id, 5);
        CHECK_EQ(Tracked::made, 1);
        CHECK_EQ(Tracked::copies + Tracked::moves, 0);
        auto b2 = b; // the closure's copy of the box shares the object
        CHECK((*b2).id == 5 && &*b2 == &*b);
        (*b2).id = 6;
        CHECK_EQ((*b).id, 6);
    }
    CHECK_EQ(Tracked::dtors, 1);

    // A type that can be neither copied nor moved is boxed too, from the prvalue that makes it.
    Pinned::dtors = 0;
    {
        auto p = gp::cgo::box([&] { return make_pinned(9); });
        CHECK_EQ((*p).v, 9);
        auto q = gp::cgo::box_as<Pinned>([&]() -> Pinned { return Pinned(10); });
        CHECK_EQ((*q).v, 10);
    }
    CHECK_EQ(Pinned::dtors, 2);

    // A move-only object is boxed and the box shared; the object lives until the last box goes.
    gp::ptr<UP> keep;
    {
        auto u = gp::cgo::box([&] { return std::make_unique<int>(41); });
        keep = u;
    }
    CHECK_EQ(**keep, 41);
    keep = nullptr;

    // box_as<T>: the declared type is the box's, the value converts.
    gp::ptr<int> n = gp::cgo::box_as<int>([&]() -> int { return int64_t(5); });
    CHECK_EQ(*n, 5);
    gp::ptr<std::vector<int64_t>> v = gp::cgo::box_as<std::vector<int64_t>>(
        [&]() -> std::vector<int64_t> { return std::vector<int64_t>{1, 2, 3}; });
    CHECK_EQ(int64_t((*v).size()), int64_t(3));

    // The copy of an lvalue is a copy (x := y).
    Tracked src(3);
    Tracked::reset();
    auto cp = gp::cgo::box([&] { return src; });
    CHECK_EQ(Tracked::copies, 1);
    CHECK_EQ((*cp).id, 3);

    // new_box<T>(): the zero value.
    gp::ptr<std::vector<int64_t>> z = gp::cgo::new_box<std::vector<int64_t>>();
    CHECK((*z).empty());
    gp::ptr<int> zi = gp::cgo::new_box<int>();
    CHECK_EQ(*zi, 0);

    // renew: a new box with a copy of the value, the old box stays with its sharers.
    auto it = gp::cgo::box([&] { return std::vector<int64_t>{7}; });
    auto old = it;
    gp::cgo::renew(it);
    (*it).push_back(8);
    CHECK_EQ(int64_t((*old).size()), int64_t(1));
    CHECK_EQ(int64_t((*it).size()), int64_t(2));
    CHECK(&*old != &*it);

    // result_of: a copy where there is one, a move for a move-only type.
    auto vv = gp::cgo::box([&] { return std::vector<int64_t>{1, 2}; });
    std::vector<int64_t> r1 = gp::cgo::result_of(*vv);
    CHECK_EQ(int64_t((*vv).size()), int64_t(2));
    CHECK_EQ(int64_t(r1.size()), int64_t(2));
    auto uu = gp::cgo::box([&] { return std::make_unique<int>(4); });
    UP r2 = gp::cgo::result_of(*uu);
    CHECK_EQ(*r2, 4);
    CHECK((*uu) == nullptr);

    // defer_arg: a copy of a variable, a moved temporary passes through.
    std::string s = "abc";
    std::string d1 = gp::cgo::defer_arg(s);
    CHECK_EQ(d1, std::string("abc"));
    CHECK_EQ(s, std::string("abc"));
    UP up = std::make_unique<int>(2);
    UP d2 = gp::cgo::defer_arg(std::move(up));
    CHECK_EQ(*d2, 2);
}

// --- result_slot ---

struct Wide { // not default-constructible: the buffer storage
    int a, b;
    Wide(int x, int y) : a(x), b(y) {}
};

Wide& global_wide() {
    static Wide w(1, 2);
    return w;
}

void test_result_slot() {
    // Default-constructible and assignable: the value itself, zero from the start.
    static_assert(sizeof(gp::result_slot<int64_t>) == sizeof(int64_t));
    static_assert(sizeof(gp::result_slot<std::string>) == sizeof(std::string));
    static_assert(sizeof(gp::result_slot<UP>) == sizeof(UP));
    gp::result_slot<int64_t> a;
    CHECK_EQ(a.gp_take(), int64_t(0)); // the zero value of an unset result
    a = 5;
    CHECK_EQ(a.gp_take(), int64_t(5));
    a.gp_set(int32_t(6)); // converts, as an assignment does
    CHECK_EQ(a.gp_take(), int64_t(6));
    gp::result_slot<std::string> s;
    s = std::string(40, 'x');
    s = std::string(41, 'y'); // set again: the last return wins
    CHECK_EQ(int64_t(s.gp_take().size()), int64_t(41));
    gp::result_slot<UP> u;
    CHECK(u.gp_take() == nullptr);
    u = std::make_unique<int>(3); // move-only
    UP out = u.gp_take();
    CHECK_EQ(*out, 3);

    // Not default-constructible: a buffer and a flag, the object made on set.
    static_assert(sizeof(gp::result_slot<Wide>) > sizeof(Wide));
    {
        gp::result_slot<Wide> w;
        w = Wide(3, 4);
        w = Wide(5, 6); // the first one is destroyed, then the second made
        Wide got = w.gp_take();
        CHECK(got.a == 5 && got.b == 6);
    }
    Tracked::reset();
    {
        gp::result_slot<Tracked> t; // no default constructor
        t = Tracked(8);
        CHECK_EQ(Tracked::copies, 0);
        Tracked got = t.gp_take();
        CHECK_EQ(got.id, 8);
        CHECK_EQ(Tracked::copies, 0);
    }
    CHECK_EQ(Tracked::dtors,
             Tracked::made + Tracked::copies + Tracked::moves); // every object ended once
    // A slot that is dropped holding a value ends it (a function that panics after a return
    // value was set).
    Tracked::reset();
    {
        gp::result_slot<Tracked> t;
        t = Tracked(9);
    }
    CHECK_EQ(Tracked::dtors, Tracked::made + Tracked::copies + Tracked::moves);
    // Not assignable (const member): the buffer too.
    static_assert(sizeof(gp::result_slot<NoAssign>) > sizeof(NoAssign));
    {
        gp::result_slot<NoAssign> n;
        NoAssign x;
        x.d = 3;
        n = x;
        CHECK_EQ(n.gp_take().d, 3);
    }

    // A reference: the object, not a copy.
    gp::result_slot<Wide&> r;
    r = global_wide();
    CHECK(&r.gp_take() == &global_wide());
    gp::result_slot<const std::string&> cr;
    std::string base = "q";
    cr = base;
    CHECK(&cr.gp_take() == &base);
}

// --- callbacks (gocpp/callback.hpp, step 4.11) ---

// The gocpp side of a C callback: unsafe.Pointer parameters, *C.char parameters, C numbers.
int cb_compare(gp::ptr<void> a, gp::ptr<void> b) {
    return *gp::unsafe_cast<int>(a) - *gp::unsafe_cast<int>(b);
}
int cb_add(gp::ptr<void> user, int x) {
    *gp::unsafe_cast<int64_t>(user) += x;
    return static_cast<int>(*gp::unsafe_cast<int64_t>(user));
}
int64_t cb_len(gp::cptr<char> s) {
    return static_cast<int64_t>(std::strlen(s.gp_get()));
}
gp::cptr<char> cb_label() {
    static char label[] = "label";
    return gp::cptr<char>(label);
}
void cb_note(int x) {
    cb_sink = x;
}
// A C function: a callback and user data (the shapes of sqlite3_exec and qsort_r).
int c_sum(int n, int (*f)(void*, int), void* user) {
    int s = 0;
    for (int i = 0; i < n; i++) {
        s += f(user, i);
    }
    return s;
}
size_t c_len(size_t (*f)(const char*), const char* s) {
    return f(s);
}
const char* c_label(const char* (*f)()) {
    return f();
}
struct cb_node {
    int v;
};
gp::ptr<void> cb_same(gp::ptr<void> u) {
    return u;
}
void* c_ud(void* (*f)(void*), void* u) {
    return f(u);
}
// A callback stored in a struct and called later, from another thread.
struct c_table {
    int (*add)(void*, int);
    void* user;
};

void test_callback() {
    // A function of the program is a C function pointer: qsort calls it.
    int a[] = {5, 2, 9, 1};
    std::qsort(a, 4, sizeof(int), gp::cgo::fn<&cb_compare>());
    CHECK_EQ(a[0] * 1000 + a[1] * 100 + a[2] * 10 + a[3], 1259);
    // The pointer is the same for the same function, and valid for ever (a static trampoline).
    int (*p1)(const void*, const void*) = gp::cgo::fn<&cb_compare>();
    int (*p2)(const void*, const void*) = gp::cgo::fn<&cb_compare>();
    CHECK(p1 == p2 && p1 != nullptr);

    // User data: a void* from C is an unsafe.Pointer that owns nothing, the same address.
    int64_t total = 100;
    CHECK_EQ(c_sum(4, gp::cgo::fn<&cb_add>(), &total), 100 + 101 + 103 + 106);
    CHECK_EQ(total, 106);

    // A *C.char parameter, and a *C.char result handed back as a raw char*.
    CHECK_EQ(static_cast<int64_t>(c_len(gp::cgo::fn<&cb_len>(), "four")), 4);
    CHECK(std::strcmp(c_label(gp::cgo::fn<&cb_label>()), "label") == 0);

    // A func literal without captures: its own trampoline, stateless.
    int64_t count = 0;
    auto twice = gp::cgo::fn_lit<gp::func<int(gp::ptr<void>, int)>>(
        []<int = 1>(gp::ptr<void> u, int x) mutable -> int {
            *gp::unsafe_cast<int64_t>(u) += 1;
            return 2 * x;
        });
    CHECK_EQ(c_sum(3, twice, &count), 0 + 2 + 4);
    CHECK_EQ(count, 3);

    // A callback table in a struct: kept by C and called later, also from a thread of C's own.
    c_table t{gp::cgo::fn<&cb_add>(), &total};
    int r = 0;
    std::thread th([&] { r = t.add(t.user, 4); });
    th.join();
    CHECK_EQ(r, 110);

    // A callback may hand back the pointer it got, also one into a struct that gocpp made.
    gp::cptr<cb_node> node = gp::cgo::box_keep<cb_node>([]() -> cb_node { return cb_node{4}; });
    CHECK(c_ud(gp::cgo::fn<&cb_same>(), node.gp_get()) == node.gp_get());

    // A void result, and a callback as a C++ callable and std::function: no trampoline.
    void (*note)(int) = gp::cgo::fn<&cb_note>();
    note(7);
    CHECK_EQ(cb_sink, 7);
    int base = 10;
    gp::func<int(int)> closure = [base](int x) { return x + base; };
    auto cb = gp::cgo::fn_val(closure);
    CHECK_EQ(cb(5), 15);
    std::function<int(int)> sf = cb;
    CHECK_EQ(sf(6), 16);
    gp::func<int(int)> back = cb;
    CHECK_EQ(back(7), 17);
    std::vector<int> v{3, 1, 2};
    std::sort(v.begin(), v.end(),
              gp::cgo::fn_lit<gp::func<bool(int, int)>>(
                  [](int x, int y) mutable -> bool { return x > y; }));
    CHECK_EQ(v[0] * 100 + v[1] * 10 + v[2], 321);

    // A panic in a callback that C++ code (not a function pointer) calls unwinds normally.
    CHECK_EQ(panic_text([&] {
                 auto boom = gp::cgo::fn_lit<gp::func<void()>>(
                     []<int = 1>() mutable -> void { gp::panic("boom"_gs); });
                 std::function<void()> f = boom;
                 f();
             }),
             std::string("boom"));
}

void run_tests() {
    test_box();
    test_result_slot();
    test_as();
    test_deref();
    test_nonnull();
    test_star_cond();
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
    test_wstr();
    test_callback();
    rt_test::threads_ended(); // see threads_ended.hpp
    if (failures > 0) {
        std::string msg =
            "FAIL: " + std::to_string(failures) + " of " + std::to_string(checks) + " checks\n";
        gp::os::write(gp::os::stderr_fd, msg.data(), msg.size());
        gp::os::exit(1);
    }
    gp::fmt::Printf("ok: %d checks\n"_gs, int64_t(checks));
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
    } else if (c == "wstrn-neg") {
        // C.GoStringWN(p, -1): the same fatal error as C.GoStringN (step 14.4).
        gp::println(gp::cgo::GoStringWN(L"hello", -1));
    } else if (c == "wstrn-neg-unsigned") {
        // An unsigned n beyond int64 counts as negative, as in GoStringN.
        gp::println(gp::cgo::GoStringWN(L"hello", ~uint64_t(0)));
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
    } else if (c == "slot-unset") {
        // A result of a type without a default constructor that was never set.
        gp::result_slot<Tracked> t;
        gp::println(t.gp_take().id);
    } else if (c == "callback-panic" || c == "callback-exception" || c == "callback-owned") {
        // A panic, a C++ exception or a gocpp-owned pointer result leaves a callback that a C
        // function (here: a function pointer call with a guard object whose destructor would
        // print) called: it must end the program at the callback, without unwinding the "C" frame.
        struct frame_guard {
            ~frame_guard() {
                const char m[] = "UNWOUND\n";
                gp::os::write(gp::os::stderr_fd, m, sizeof(m) - 1);
            }
        };
        auto call_c = [](int (*f)(void*, int)) {
            frame_guard g;
            return f(nullptr, 1);
        };
        if (c == "callback-panic") {
            call_c(gp::cgo::fn_lit<gp::func<int(gp::ptr<void>, int)>>(
                []<int = 1>(gp::ptr<void>, int) mutable -> int { gp::panic("stop"_gs); }));
        } else if (c == "callback-exception") {
            call_c(gp::cgo::fn_lit<gp::func<int(gp::ptr<void>, int)>>(
                []<int = 1>(gp::ptr<void>, int) mutable -> int {
                    throw std::runtime_error("stop");
                }));
        } else {
            int* (*f)() = gp::cgo::fn_lit<gp::func<gp::ptr<int>()>>(
                []<int = 1>() mutable -> gp::ptr<int> { return gp::new_<int>(7); });
            gp::println(f() == nullptr);
        }
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
