// The unit tests of package unsafe (gocpp/unsafe.hpp, Phase 04 §2.7, step 4.10): unsafe.Pointer
// as an owning gp::ptr<void>, unsafe.Add, SliceData, Slice, String and StringData, the
// conversions of C pointers and functions (gp::cgo::as), Go's panics, and the owners that keep
// memory alive. runtime/cxx_test.go compiles this file with the runtime's .cpp files, with g++
// and clang++, at -O0, at -O2 and with ASan + UBSan (LeakSanitizer included), and runs it like
// test/unit.cpp: it prints "ok: N checks" and exits 0, or prints each failed check and exits 1.
// With an argument it runs one case that ends the program instead (the cases at the end of the
// file), which cxx_test.go checks from the outside.
//
// Under ASan the tests that drop the owner first and read afterwards (the Builder, the slices
// and the strings below) are the proof that no view dangles: a pointer that owned nothing would
// be a heap-use-after-free there.
#include <gocpp/rt.hpp>
#include <gocpp/os.hpp>

#include <array>
#include <chrono>
#include <cstddef>
#include <cstdint>
#include <cstring>
#include <cstdlib>
#include <memory>
#include <string>
#include <type_traits>
#include <utility>

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

struct Pair {
    int32_t a = 0;
    int32_t b = 0;
};

int add(int x, int y) {
    return x + y;
}

// hide returns p, which the compiler can't see through: a string over a three-byte array would
// make g++ -O2 warn (-Warray-bounds) about the read of the byte after it that gp_c_terminated
// never makes for a string with an owner.
[[gnu::noinline]] const char* hide(const char* p) {
    return p;
}

// --- unsafe.Pointer over gocpp memory ---

void test_pointer() {
    gp::ptr<int64_t> p = gp::new_<int64_t>(7);
    CHECK_EQ(int64_t(p.gp_shared().use_count()), int64_t(1));
    gp::ptr<void> u = gp::unsafe_pointer(p);
    CHECK(u.gp_get() == p.gp_get());
    CHECK_EQ(int64_t(p.gp_shared().use_count()),
             int64_t(2)); // the unsafe.Pointer owns the object too
    gp::ptr<int64_t> q = gp::unsafe_cast<int64_t>(u);
    CHECK(q == p);
    CHECK_EQ(int64_t(p.gp_shared().use_count()), int64_t(3));
    *q = 9;
    CHECK_EQ(*p, int64_t(9));

    // The object lives as long as the unsafe.Pointer does: drop the typed pointers first.
    gp::ptr<void> keep = gp::unsafe_pointer(std::move(p));
    CHECK(p == nullptr);
    q = nullptr;
    u = nullptr;
    CHECK_EQ(*gp::unsafe_cast<int64_t>(keep), int64_t(9)); // ASan: the block is still there
    CHECK_EQ(int64_t(keep.gp_shared().use_count()), int64_t(1));

    // nil stays nil, in both directions.
    CHECK(gp::unsafe_pointer(gp::ptr<int64_t>()) == nullptr);
    CHECK(gp::unsafe_cast<int64_t>(gp::ptr<void>()) == nullptr);
    CHECK(gp::unsafe_pointer(gp::ptr<void>(keep)) == keep);

    // The conversion to void* for C is the raw address, borrowed.
    void* raw = keep;
    CHECK(raw == keep.gp_get());

    // A pointer to a field shares the block of the struct, and so does its unsafe.Pointer.
    gp::ptr<Pair> s = gp::new_<Pair>(Pair{1, 2});
    gp::ptr<int32_t> pb(s, &s->b);
    gp::ptr<void> ub = gp::unsafe_pointer(pb);
    s = nullptr;
    pb = nullptr;
    CHECK_EQ(*gp::unsafe_cast<int32_t>(ub), int32_t(2));

    // unsafe.Pointer is comparable and hashable by address.
    gp::ptr<void> a = gp::unsafe_pointer(gp::new_<int64_t>(1));
    gp::ptr<void> b = a;
    CHECK(a == b);
    CHECK(a != gp::unsafe_pointer(gp::new_<int64_t>(1)));
    CHECK(std::hash<gp::ptr<void>>()(a) == std::hash<gp::ptr<void>>()(b));
}

void test_addr() {
    gp::slice<int64_t> s = gp::slice_of<int64_t>({10, 20, 30, 40});
    gp::ptr<void> p = gp::unsafe_pointer(gp::unsafe_slice_data(s));
    std::uintptr_t addr = gp::unsafe_addr(p);
    CHECK(addr == reinterpret_cast<std::uintptr_t>(&s[0]));
    CHECK_EQ(gp::unsafe_addr(gp::ptr<void>()), std::uintptr_t(0));
    // unsafe.Pointer(uintptr(p) + 8): the pattern of go vet's rule 3.
    gp::ptr<void> q = gp::unsafe_from_addr(addr + 8);
    CHECK_EQ(*gp::unsafe_cast<int64_t>(q), int64_t(20));
    CHECK(gp::unsafe_from_addr(0) == nullptr);
    // A pointer made from a number owns nothing (the foreign owner), so it keeps nothing alive.
    CHECK(q.gp_shared().get() == q.gp_get());
    CHECK(q.gp_shared().use_count() > 1);
}

void test_add() {
    gp::slice<int64_t> s = gp::slice_of<int64_t>({10, 20, 30, 40});
    gp::ptr<void> p = gp::unsafe_pointer(gp::unsafe_slice_data(s));
    gp::ptr<void> q = gp::unsafe_add(p, 16);
    CHECK_EQ(*gp::unsafe_cast<int64_t>(q), int64_t(30));
    gp::ptr<void> back = gp::unsafe_add(q, -8);
    CHECK_EQ(*gp::unsafe_cast<int64_t>(back), int64_t(20));
    CHECK(gp::unsafe_add(p, 0) == p);
    CHECK(gp::unsafe_add(p, uint8_t(24)).gp_get() == &s[3]);
    // The result keeps the array alive.
    s = nullptr;
    p = nullptr;
    CHECK_EQ(*gp::unsafe_cast<int64_t>(q), int64_t(30));
    CHECK(gp::unsafe_add(gp::ptr<void>(), 0) == nullptr);
    CHECK(gp::unsafe_add(gp::ptr<void>(), 8).gp_get() == reinterpret_cast<void*>(8));
}

// --- SliceData, Slice ---

void test_slice_data() {
    gp::slice<int64_t> nil;
    CHECK(gp::unsafe_slice_data(nil) == nullptr);
    gp::slice<int64_t> empty = gp::make_slice<int64_t>(0);
    CHECK(gp::unsafe_slice_data(empty) != nullptr); // an empty slice that is not nil
    gp::slice<int64_t> s = gp::slice_of<int64_t>({1, 2, 3});
    gp::ptr<int64_t> p = gp::unsafe_slice_data(s);
    CHECK(p.gp_get() == &s[0]);
    CHECK(p.gp_shared().use_count() == s.gp_owner().use_count());
    // s[1:] starts at its own first element.
    gp::slice<int64_t> t = gp::sub(s, 1);
    CHECK(gp::unsafe_slice_data(t).gp_get() == &s[1]);
    // The pointer keeps the array alive after the slices are gone.
    s = nullptr;
    t = nullptr;
    *p = 5;
    CHECK_EQ(*p, int64_t(5));
}

void test_slice() {
    gp::slice<int64_t> s = gp::slice_of<int64_t>({1, 2, 3, 4, 5});
    gp::ptr<int64_t> p(s.gp_owner(), &s[1]);
    gp::slice<int64_t> t = gp::unsafe_slice(p, 3);
    CHECK_EQ(gp::len(t), int64_t(3));
    CHECK_EQ(gp::cap(t), int64_t(3)); // Go: the capacity is the length
    CHECK(t[0] == 2 && t[2] == 4);
    t[0] = 20; // the same array
    CHECK_EQ(s[1], int64_t(20));
    // The slice shares p's owner.
    s = nullptr;
    p = nullptr;
    CHECK_EQ(t[1], int64_t(3));

    // A moved pointer gives its owner to the slice (no copy of the count).
    gp::slice<int64_t> a = gp::slice_of<int64_t>({7, 8});
    gp::slice<int64_t> b = gp::unsafe_slice(gp::unsafe_slice_data(a), 2);
    CHECK(a.gp_owner().get() == b.gp_owner().get());
    CHECK_EQ(int64_t(a.gp_owner().use_count()), int64_t(2));

    // A length of 0: nil for a nil pointer, else an empty slice that is not nil.
    CHECK(gp::unsafe_slice(gp::ptr<int64_t>(), 0) == nullptr);
    gp::slice<int64_t> z = gp::unsafe_slice(gp::unsafe_slice_data(a), 0);
    CHECK(z != nullptr && gp::len(z) == 0);

    // The elements of a gocpp pointer may be any type.
    gp::slice<gp::string> names = gp::slice_of<gp::string>({"a"_gs, "b"_gs});
    gp::slice<gp::string> same = gp::unsafe_slice(gp::unsafe_slice_data(names), 2);
    CHECK(same[1] == "b"_gs);

    // Go's panics.
    CHECK_EQ(panic_text([&] { gp::unsafe_slice(gp::unsafe_slice_data(a), -1); }),
             std::string("runtime error: unsafe.Slice: len out of range"));
    CHECK_EQ(panic_text([&] { gp::unsafe_slice(gp::ptr<int64_t>(), 1); }),
             std::string("runtime error: unsafe.Slice: ptr is nil and len is not zero"));
    CHECK_EQ(panic_text([&] { gp::unsafe_slice(gp::unsafe_slice_data(a), INT64_MAX / 4); }),
             std::string("runtime error: unsafe.Slice: len out of range")); // 8 * len wraps
    CHECK_EQ(panic_text([&] { gp::unsafe_slice(gp::unsafe_slice_data(a), UINT64_MAX); }),
             std::string("runtime error: unsafe.Slice: len out of range"));
    CHECK_EQ(panic_text([&] { gp::unsafe_slice(gp::unsafe_slice_data(a), 2); }),
             std::string("<no panic>"));
}

void test_slice_c() {
    int arr[5] = {1, 2, 3, 4, 5};
    gp::slice<int> s = gp::unsafe_slice(&arr[1], 3);
    CHECK_EQ(gp::len(s), int64_t(3));
    CHECK(s[0] == 2 && s[2] == 4);
    s[1] = 30; // C memory: the same bytes
    CHECK_EQ(arr[2], 30);
    CHECK(s.gp_data() == &arr[1]);
    // const is dropped, as in cgo.
    const int* cp = arr;
    gp::slice<int> c = gp::unsafe_slice(cp, 5);
    CHECK(c.gp_data() == arr && gp::len(c) == 5);
    // A pointer to a C struct, and a pointer to a pointer (char** argv).
    Pair pairs[2] = {{1, 2}, {3, 4}};
    gp::slice<Pair> ps = gp::unsafe_slice(pairs, 2);
    CHECK(ps[1].b == 4);
    const char* words[2] = {"x", "yz"};
    gp::slice<const char*> ws = gp::unsafe_slice(words, 2);
    CHECK(std::strlen(ws[1]) == 2);
    // The C slice owns nothing, so it is cheap to copy and can't free the C array.
    gp::slice<int> copy = s;
    CHECK(copy.gp_owner() == gp::foreign_owner());
    // nil and the panics.
    CHECK(gp::unsafe_slice(static_cast<int*>(nullptr), 0) == nullptr);
    CHECK_EQ(panic_text([&] { gp::unsafe_slice(static_cast<int*>(nullptr), 2); }),
             std::string("runtime error: unsafe.Slice: ptr is nil and len is not zero"));
    CHECK_EQ(panic_text([&] { gp::unsafe_slice(arr, -3); }),
             std::string("runtime error: unsafe.Slice: len out of range"));
    // A slice of C elements converts to std::span, like any slice.
    std::span<int> sp = s;
    CHECK(sp.size() == 3 && sp[0] == 2);
}

// --- String, StringData ---

void test_string_data() {
    gp::string lit = "hello"_gs;
    gp::ptr<uint8_t> p = gp::unsafe_string_data(lit);
    CHECK(p != nullptr);
    CHECK(p.gp_get() == reinterpret_cast<const uint8_t*>(lit.gp_data()));
    CHECK(*p == 'h');
    CHECK(gp::unsafe_string_data(gp::string()) == nullptr);
    CHECK(gp::unsafe_string_data("x"_gs) != nullptr);

    // A string made at run time: the pointer keeps its bytes after the string is gone.
    gp::string s = gp::concat("hello, "_gs, "world"_gs);
    gp::ptr<uint8_t> q = gp::unsafe_string_data(s);
    CHECK_EQ(int64_t(q.gp_shared().use_count()), int64_t(2)); // the string and the pointer
    s = gp::string();
    CHECK(q.gp_get()[7] == 'w');
    CHECK(std::memcmp(q.gp_get(), "hello, world", 12) == 0);
    // And a string over the pointer keeps them too.
    gp::string again = gp::unsafe_string(q, 12);
    q = nullptr;
    CHECK_EQ(str(again), std::string("hello, world"));
}

void test_string() {
    gp::slice<uint8_t> b = gp::make_slice<uint8_t>(0, 16);
    b = gp::append(std::move(b), uint8_t('g'), uint8_t('o'), uint8_t('!'));
    gp::string s = gp::unsafe_string(gp::unsafe_slice_data(b), gp::len(b));
    CHECK_EQ(str(s), std::string("go!"));
    CHECK(s.gp_data() == reinterpret_cast<const char*>(b.gp_data())); // no copy
    CHECK(!s.gp_c_terminated()); // it has an owner, so no NUL is promised after it
    // The string keeps the array alive.
    b = nullptr;
    CHECK_EQ(str(s), std::string("go!"));
    gp::string sub = gp::sub(s, 1);
    s = gp::string();
    CHECK_EQ(str(sub), std::string("o!"));

    CHECK_EQ(str(gp::unsafe_string(gp::ptr<uint8_t>(), 0)), std::string(""));
    CHECK_EQ(gp::len(gp::unsafe_string(gp::ptr<uint8_t>(), 0)), int64_t(0));
    gp::slice<uint8_t> a = gp::make_slice<uint8_t>(4);
    CHECK_EQ(panic_text([&] { gp::unsafe_string(gp::unsafe_slice_data(a), -1); }),
             std::string("runtime error: unsafe.String: len out of range"));
    CHECK_EQ(panic_text([&] { gp::unsafe_string(gp::ptr<uint8_t>(), 1); }),
             std::string("runtime error: unsafe.String: ptr is nil and len is not zero"));
    CHECK_EQ(panic_text([&] { gp::unsafe_string(gp::unsafe_slice_data(a), UINT64_MAX); }),
             std::string("runtime error: unsafe.String: len out of range"));
    CHECK_EQ(panic_text([&] { gp::unsafe_string(gp::unsafe_slice_data(a), 4); }),
             std::string("<no panic>"));
}

void test_string_c() {
    // C memory that has no NUL after its bytes: an array of exactly three characters.
    std::array<char, 3> buf = {'a', 'b', 'c'};
    gp::string s = gp::unsafe_string(hide(buf.data()), 3);
    CHECK_EQ(str(s), std::string("abc"));
    CHECK(s.gp_data() == buf.data());
    // The owner is the foreign one, not null: a string without an owner would promise a NUL at
    // data[len], and gp::cstr would read it (ASan: stack-buffer-overflow).
    CHECK(!s.gp_c_terminated());
    CHECK(s.gp_owner() == gp::foreign_owner());
    gp::cstr_holder h = gp::cstr(s);
    CHECK(std::strcmp(h.gp_ptr(), "abc") == 0); // a copy with its own NUL
    CHECK(h.gp_ptr() != buf.data());
    const unsigned char ubuf[2] = {'o', 'k'};
    CHECK_EQ(str(gp::unsafe_string(ubuf, 2)), std::string("ok"));
    CHECK_EQ(str(gp::unsafe_string(static_cast<const char*>(nullptr), 0)), std::string(""));
    CHECK_EQ(panic_text([&] { gp::unsafe_string(static_cast<const char*>(nullptr), 3); }),
             std::string("runtime error: unsafe.String: ptr is nil and len is not zero"));
}

// --- Conversions of C values ---

void test_cgo_as() {
    int x = 5;
    gp::ptr<void> p = gp::cgo::as<gp::ptr<void>>(&x);
    CHECK(p.gp_get() == &x);
    CHECK(p.gp_shared() == gp::foreign_owner() || p.gp_shared().get() == p.gp_get());
    CHECK(gp::cgo::as<gp::ptr<void>>(static_cast<int*>(nullptr)) == nullptr);
    CHECK(gp::cgo::as<gp::ptr<void>>(nullptr) == nullptr);
    const char* cs = "text";
    CHECK(gp::cgo::as<gp::ptr<void>>(cs).gp_get() == cs);
    void* v = &x;
    CHECK(gp::cgo::as<gp::ptr<void>>(v).gp_get() == &x);
    // A gocpp pointer converts too (the result of unsafe.SliceData, which stays opaque).
    gp::ptr<int64_t> g = gp::new_<int64_t>(3);
    gp::ptr<void> u = gp::cgo::as<gp::ptr<void>>(g);
    CHECK(u.gp_get() == g.gp_get() && g.gp_shared().use_count() == 2);

    // And back to a C pointer.
    int* ip = gp::cgo::as<int*>(p);
    CHECK(ip == &x);
    *ip = 6;
    CHECK_EQ(x, 6);
    const char* back = gp::cgo::as<const char*>(gp::cgo::as<gp::ptr<void>>(cs));
    CHECK(back == cs);
    void* vp = gp::cgo::as<void*>(p);
    CHECK(vp == &x);
    CHECK(gp::cgo::as<int*>(gp::ptr<void>()) == nullptr);
    Pair pair{3, 4};
    Pair* pp = gp::cgo::as<Pair*>(gp::cgo::as<gp::ptr<void>>(&pair));
    CHECK(pp->b == 4);

    // A C function through unsafe.Pointer and back: std::bit_cast, no object-pointer cast.
    gp::ptr<void> fp = gp::cgo::as<gp::ptr<void>>(add);
    CHECK(fp != nullptr);
    auto fn = gp::cgo::as<int (*)(int, int)>(fp);
    CHECK_EQ(fn(2, 3), 5);
    CHECK(fn == &add);
    gp::ptr<void> fp2 = gp::cgo::as<gp::ptr<void>>(&add);
    CHECK(fp2 == fp);

    // An address as a number.
    CHECK(gp::cgo::as<gp::ptr<void>>(reinterpret_cast<std::uintptr_t>(&x)).gp_get() == &x);
}

// --- cptr, out_ptr (the owning pointers of C types) ---

struct Node {
    int v = 0;
    Node* next = nullptr;
};

struct Pinned {
    int v;
    explicit Pinned(int x) : v(x) {}
    Pinned(const Pinned&) = delete;
    Pinned(Pinned&&) = delete;
};

void alloc_out(void** out) {
    *out = std::malloc(8);
    std::memset(*out, 'q', 8);
}

// owned<T>(make) is &T{...} of a C type: a box that builds the T in place, as a gp::cptr.
template <typename T, typename F>
gp::cptr<T> owned(F&& make) {
    return gp::cgo::box_keep<T>(std::forward<F>(make));
}

void test_cptr() {
    // &T{...}: built in place (no move), owns the object, converts to the raw pointer for C.
    auto a = owned<Node>([] { return Node{1, nullptr}; });
    auto b = owned<Node>([&] { return Node{2, a}; }); // a converts to Node*
    CHECK_EQ(int64_t(a.gp_shared().use_count()), int64_t(1));
    CHECK_EQ(b.gp_get()->next->v, 1);
    // A list walk: p starts as an owning pointer, and takes the raw pointers of next.
    gp::cptr<Node> p = b;
    CHECK_EQ(int64_t(b.gp_shared().use_count()), int64_t(2));
    int sum = 0;
    for (; p != nullptr; p = p->next) {
        sum += p->v;
    }
    CHECK_EQ(sum, 3);
    CHECK(p == nullptr);
    CHECK_EQ(int64_t(b.gp_shared().use_count()), int64_t(1)); // p let go of b
    gp::cptr<Node> q = a.gp_get(); // a raw pointer to an object that gocpp owns: owns it again
    CHECK(!q.gp_shared().owner_before(a.gp_shared()) && !a.gp_shared().owner_before(q.gp_shared()));
    Node cnode{9, nullptr};
    gp::cptr<Node> fromc = &cnode; // from C: owns nothing
    CHECK(!fromc.gp_shared().owner_before(gp::foreign_owner()) &&
          !gp::foreign_owner().owner_before(fromc.gp_shared()));
    CHECK(q == a.gp_get() && q == a);
    q = nullptr;
    CHECK(q == nullptr);
    Node* raw = b; // borrowed
    CHECK(raw == b.gp_get());
    gp::cptr<Node> none = static_cast<Node*>(nullptr);
    CHECK(none == nullptr && none.gp_get() == nullptr);

    // No copy or move constructor needed.
    auto m = owned<Pinned>([] { return Pinned(5); });
    CHECK_EQ(m.gp_get()->v, 5);

    // It converts to unsafe.Pointer and keeps the object alive.
    gp::ptr<void> u = gp::unsafe_pointer(a);
    a = nullptr;
    CHECK_EQ(static_cast<Node*>(u.gp_get())->v, 1);
    gp::slice<Node> sl = gp::unsafe_slice(gp::unsafe_cast<Node>(u), 1);
    u = nullptr;
    CHECK_EQ(sl[0].v, 1);
}

// same_owner reports whether two shared pointers share one control block (shared_ptr's == compares
// the stored addresses, which an aliasing pointer to the same object has in common with a pointer
// that owns nothing of it).
template <typename A, typename B>
bool same_owner(const std::shared_ptr<A>& a, const std::shared_ptr<B>& b) {
    return !a.owner_before(b) && !b.owner_before(a);
}

void test_out_cptr() {
    // A T** out-parameter of a gp::cptr variable (sqlite3_open's &db).
    auto alloc_int = [](int** out) {
        *out = static_cast<int*>(std::malloc(sizeof(int)));
        **out = 42;
    };
    gp::cptr<int> p;
    alloc_int(gp::cgo::out_ptr(p));
    CHECK(p != nullptr && *p.gp_get() == 42);
    CHECK(same_owner(p.gp_shared(), gp::foreign_owner())); // from C: owns nothing
    std::free(p.gp_get());
    // A function that writes nothing leaves the owner alone.
    gp::cptr<int64_t> owned(gp::new_<int64_t>(5));
    auto noop = [](int64_t**) {};
    noop(gp::cgo::out_ptr(owned));
    CHECK(owned != nullptr && !same_owner(owned.gp_shared(), gp::foreign_owner()));
    // A raw pointer variable: its own address.
    int* raw = nullptr;
    alloc_int(gp::cgo::out_ptr(raw));
    CHECK(raw != nullptr && *raw == 42);
    std::free(raw);
    // raw(x): a gocpp pointer as its address, anything else as it is.
    gp::cptr<int64_t> q(gp::new_<int64_t>(7));
    static_assert(std::is_pointer_v<decltype(gp::cgo::raw(q))>); // a raw address, not q itself
    CHECK(gp::cgo::raw(q) == q.gp_get());
    // The constructors of cptr from a const gp::ptr keep its owner, as the rvalue one does.
    const gp::ptr<int64_t> cp = gp::new_<int64_t>(11);
    gp::cptr<int64_t> viaCopy(cp);
    CHECK(same_owner(viaCopy.gp_shared(), cp.gp_shared()) &&
          !same_owner(viaCopy.gp_shared(), gp::foreign_owner()));
    gp::cptr<int64_t> assigned;
    assigned = cp;
    CHECK(same_owner(assigned.gp_shared(), cp.gp_shared()));
    int n = 3;
    CHECK(gp::cgo::raw(n) == 3);
}

void test_box_keep() {
    // A node that holds a raw pointer to another keeps the other's object alive (a member of a C
    // struct is raw: the box owns the object it points to). The lambda gets the kept pointers.
    gp::cptr<Node> head = owned<Node>([] { return Node{1, nullptr}; });
    std::weak_ptr<Node> w = head.gp_shared();
    gp::cptr<Node> n =
        gp::cgo::box_keep<Node>([](const gp::cptr<Node>& h) -> Node { return Node{2, h}; }, head);
    head = nullptr; // the only other owner of the first node
    CHECK(!w.expired());
    CHECK_EQ(n.gp_get()->next->v, 1); // ASan: not freed
    n = nullptr;
    CHECK(w.expired());
    // A raw pointer owns nothing, so there is nothing to keep.
    Node* rawp = nullptr;
    gp::cptr<Node> m = gp::cgo::box_keep<Node>([](Node* r) -> Node { return Node{3, r}; }, rawp);
    CHECK_EQ(m.gp_get()->v, 3);
    // The kept pointer of a temporary: &lib.Node{next: &lib.Node{}}.
    gp::cptr<Node> t =
        gp::cgo::box_keep<Node>([](const gp::cptr<Node>& h) -> Node { return Node{4, h}; },
                                owned<Node>([] { return Node{5, nullptr}; }));
    CHECK_EQ(t.gp_get()->next->v, 5); // ASan: the temporary lives on in t
    t = nullptr;
}

// Chain and array members of a struct, for store_keep.
struct Big {
    Node* slots[1000] = {};
};
struct Inner {
    Node* p = nullptr;
};
struct Outer {
    int v = 0;
    Inner in;
    Node* slots[3] = {};
    void* data = nullptr;
};

void test_store_keep() {
    // n.next = head: the box of n keeps head's object after head goes.
    auto n = owned<Node>([] { return Node{1, nullptr}; });
    auto a = owned<Node>([] { return Node{2, nullptr}; });
    std::weak_ptr<Node> wa = a.gp_shared();
    gp::cgo::store_keep(n, n->next, a);
    a = nullptr;
    CHECK(!wa.expired());
    CHECK_EQ(n->next->v, 2); // ASan: a use after free if it was not kept
    // A store replaces what the member kept (one entry per member), and nil drops it.
    auto b = owned<Node>([] { return Node{3, nullptr}; });
    std::weak_ptr<Node> wb = b.gp_shared();
    gp::cgo::store_keep(n, n->next, b);
    CHECK(wa.expired()); // the old object is released by the new store
    b = nullptr;
    CHECK(!wb.expired() && n->next->v == 3);
    gp::cgo::store_keep(n, n->next, nullptr);
    CHECK(wb.expired() && n->next == nullptr);
    // A raw pointer drops the entry too, and owns nothing.
    auto c = owned<Node>([] { return Node{4, nullptr}; });
    std::weak_ptr<Node> wc = c.gp_shared();
    gp::cgo::store_keep(n, n->next, c);
    c = nullptr;
    CHECK(!wc.expired());
    Node local{5, nullptr};
    gp::cgo::store_keep(n, n->next, &local);
    CHECK(wc.expired() && n->next == &local);
    gp::cgo::store_keep(n, n->next, nullptr);

    // A loop that stores into one member holds one object at a time.
    std::weak_ptr<Node> prev;
    for (int i = 0; i < 100; i++) {
        auto x = owned<Node>([i] { return Node{i, nullptr}; });
        std::weak_ptr<Node> cur = x.gp_shared();
        gp::cgo::store_keep(n, n->next, x);
        CHECK(i == 0 || prev.expired()); // the store released the previous object
        prev = cur;
    }
    CHECK(!prev.expired());
    CHECK_EQ(n->next->v, 99);
    // The object dies with n: what it kept is released.
    n = nullptr;
    CHECK(prev.expired());

    // Elements of an array member, and a member through a struct member (a chain).
    auto o = owned<Outer>([] { return Outer{}; });
    auto s0 = owned<Node>([] { return Node{10, nullptr}; });
    auto s1 = owned<Node>([] { return Node{11, nullptr}; });
    auto in = owned<Node>([] { return Node{12, nullptr}; });
    std::weak_ptr<Node> w0 = s0.gp_shared(), w1 = s1.gp_shared(), win = in.gp_shared();
    gp::cgo::store_keep(o, o->slots[0], s0);
    gp::cgo::store_keep(o, o->slots[1], s1);
    gp::cgo::store_keep(o, o->in.p, in, gp::cgo::keep_path<decltype(o->in)>());
    s0 = s1 = in = nullptr;
    CHECK(!w0.expired() && !w1.expired() && !win.expired());
    CHECK_EQ(o->slots[0]->v + o->slots[1]->v + o->in.p->v, 33);
    gp::cgo::store_keep(o, o->slots[0], nullptr); // another slot's entry stays
    CHECK(w0.expired() && !w1.expired());
    // An unsafe.Pointer stored in a void* member keeps its object too.
    auto d = owned<Node>([] { return Node{13, nullptr}; });
    std::weak_ptr<Node> wd = d.gp_shared();
    gp::cgo::store_keep(o, o->data, gp::unsafe_pointer(d));
    d = nullptr;
    CHECK(!wd.expired() && static_cast<Node*>(o->data)->v == 13);
    // A member that is no pointer just gets the value.
    gp::cgo::store_keep(o, o->v, 7);
    CHECK_EQ(o->v, 7);
    o = nullptr;
    CHECK(w1.expired() && win.expired() && wd.expired());

    // Stores of nil, and of a pointer that is C's (foreign owner), into C memory need no owner.
    Node cnode{20, nullptr};
    gp::cptr<Node> cp = &cnode;
    gp::cgo::store_keep(cp, cp->next, nullptr);
    gp::cgo::store_keep(cp, cp->next, gp::cptr<Node>(&local));
    CHECK(cnode.next == &local);
    gp::cgo::store_keep(cp, cp->next, &local);
}

void test_sizes() {
    struct Wide {
        char c;
        double d;
    };
    int arr[3] = {};
    CHECK_EQ(gp::unsafe_sizeof<Wide>(), std::uintptr_t(sizeof(Wide)));
    CHECK_EQ(gp::unsafe_alignof<Wide>(), std::uintptr_t(alignof(double)));
    CHECK_EQ(gp::unsafe_sizeof<decltype(arr)>(), std::uintptr_t(3 * sizeof(int)));
    CHECK_EQ(gp::unsafe_sizeof<int&>(), std::uintptr_t(sizeof(int)));
    CHECK_EQ(gp::unsafe_sizeof<char>(), std::uintptr_t(1)); // not the size of a pointer
    CHECK(gp::unsafe_sizeof<Wide*>() != gp::unsafe_sizeof<Wide>());
}

// Reading a raw pointer member back: a raw T* to an object that gocpp owns converts to a pointer
// that owns it (the registry), so a variable that takes the member keeps the object alive.
void test_read_back() {
    auto build = [](int n) { // 1 -> 2 -> ... -> n, each node holds the next one
        gp::cptr<Node> head;
        for (int i = n; i >= 1; i--) {
            auto x = owned<Node>([i] { return Node{i, nullptr}; });
            gp::cgo::store_keep(x, x->next, head);
            head = x;
        }
        return head;
    };
    // Pop: head = head.next; the first node goes, the second lives on.
    {
        gp::cptr<Node> head = build(3);
        std::weak_ptr<Node> w1 = head.gp_shared();
        head = head->next; // raw -> cptr
        CHECK(w1.expired());
        CHECK_EQ(head->v, 2);
        CHECK_EQ(head->next->v, 3);
    }
    // Unlink: n.next = other.next; other.next = nil; other = nil.
    {
        auto n = owned<Node>([] { return Node{1, nullptr}; });
        gp::cptr<Node> other = build(3); // 1 -> 2 -> 3
        std::weak_ptr<Node> w2 = std::shared_ptr<Node>(other.gp_shared(), other->next);
        std::weak_ptr<Node> wo = other.gp_shared();
        (void)w2;
        gp::cgo::store_keep(n, n->next, other->next); // a raw pointer: the registry finds its owner
        gp::cgo::store_keep(other, other->next, nullptr);
        other = nullptr;
        CHECK(wo.expired());
        CHECK_EQ(n->next->v, 2);
        CHECK_EQ(n->next->next->v, 3);
    }
    // Reverse in place with auto next = load(cur.next), as the emitter writes it.
    {
        gp::cptr<Node> head = build(5);
        gp::cptr<Node> prev;
        for (gp::cptr<Node> cur = head; cur != nullptr;) {
            auto next = gp::cgo::load(cur->next);
            gp::cgo::store_keep(cur, cur->next, prev);
            prev = cur;
            cur = next;
        }
        head = nullptr;
        int sum = 0, first = prev->v;
        for (gp::cptr<Node> p = prev; p != nullptr; p = p->next) {
            sum += p->v;
        }
        CHECK_EQ(sum, 15);
        CHECK_EQ(first, 5);
    }
    // Swap two members through temporaries, and move a node between lists.
    {
        gp::cptr<Node> a = build(2), b = build(3);
        auto ta = gp::cgo::load(a->next), tb = gp::cgo::load(b->next);
        gp::cgo::store_keep(a, a->next, tb);
        gp::cgo::store_keep(b, b->next, ta);
        ta = tb = nullptr;
        CHECK_EQ(a->next->v, 2);
        CHECK_EQ(b->next->v, 2);
        // The first of b's tail moves to the front of a.
        auto moved = gp::cgo::load(b->next);
        gp::cgo::store_keep(b, b->next, moved->next);
        gp::cgo::store_keep(moved, moved->next, a);
        a = moved;
        moved = nullptr;
        CHECK_EQ(a->v + a->next->v, 2 + 1);
    }
    // load of a member that is no pointer is the value itself; a pointer to a non-class too.
    {
        Outer o;
        o.v = 4;
        CHECK_EQ(gp::cgo::load(o.v), 4);
        int x = 0;
        o.data = &x;
        CHECK(gp::cgo::load(o.data) == &x);
        static_assert(
            std::is_same_v<decltype(gp::cgo::load(std::declval<Node*&>())), gp::cptr<Node>>);
    }
}

// Structs that have no keeper of their own: a C struct in a slice, in a gocpp struct, and a
// pointer that aliases a part of a block; they keep through the table of the block's owner.
struct Wrapper {
    int tag = 0;
    Node c;
};

void test_side_keep() {
    // nodes[i].next = x keeps x as long as the slice lives; &nodes[i+1] needs nothing (one
    // allocation owns both), so the slice is not kept alive by itself.
    std::weak_ptr<void> wslice;
    std::weak_ptr<Node> wx;
    {
        gp::slice<Node> nodes = gp::make_slice<Node>(int64_t(4));
        auto x = owned<Node>([] { return Node{50, nullptr}; });
        wx = x.gp_shared();
        gp::cgo::store_keep(nodes, nodes[0].next, x);
        gp::cptr<Node> self(gp::ptr<Node>(nodes.gp_owner(), &nodes[2]));
        gp::cgo::store_keep(nodes, nodes[1].next, self); // aliases the slice's own block
        x = nullptr;
        CHECK(!wx.expired());
        CHECK_EQ(nodes[0].next->v, 50);
        wslice = nodes.gp_owner();
        CHECK(nodes[1].next == &nodes[2]);
    }
    CHECK(wslice.expired()); // no self-reference cycle
    // A C struct field of a gocpp struct (s.c.next = x), through the pointer to the gocpp struct.
    {
        gp::ptr<Wrapper> s = gp::new_<Wrapper>();
        auto x = owned<Node>([] { return Node{60, nullptr}; });
        wx = x.gp_shared();
        gp::cgo::store_keep(s, s->c.next, x);
        x = nullptr;
        CHECK(!wx.expired() && s->c.next->v == 60);
        gp::cgo::store_keep(s, s->c.next, nullptr); // replaced: released at once
        CHECK(wx.expired());
        auto y = owned<Node>([] { return Node{61, nullptr}; });
        wx = y.gp_shared();
        gp::cgo::store_keep(s, s->c.next, y);
        y = nullptr;
        s = nullptr; // the gocpp struct dies: what it kept is dropped by the table's next purge
        // Many other blocks make the table purge (it doubles at 64 entries).
        for (int i = 0; i < 200; i++) {
            gp::ptr<Wrapper> t = gp::new_<Wrapper>();
            auto z = owned<Node>([] { return Node{0, nullptr}; });
            gp::cgo::store_keep(t, t->c.next, z);
        }
        CHECK(wx.expired());
    }
    // A pointer that aliases a gocpp struct's field (&s.c): the block is the struct's.
    {
        gp::ptr<Wrapper> s = gp::new_<Wrapper>();
        gp::cptr<Node> alias(gp::ptr<Node>(s, &s->c));
        auto x = owned<Node>([] { return Node{70, nullptr}; });
        std::weak_ptr<Node> w = x.gp_shared();
        gp::cgo::store_keep(alias, alias->next, x);
        x = nullptr;
        CHECK(!w.expired() && s->c.next->v == 70);
    }
}

// Freeing a long list that holds each next through a keeper must not recurse a million deep.
void test_long_list() {
#if defined(__SANITIZE_ADDRESS__)
    constexpr int n = 200000;
#elif defined(__has_feature)
#if __has_feature(address_sanitizer)
    constexpr int n = 200000;
#else
    constexpr int n = 1000000;
#endif
#else
    constexpr int n = 1000000;
#endif
    {
        gp::cptr<Node> head;
        for (int i = 0; i < n; i++) {
            auto x = owned<Node>([i] { return Node{i, nullptr}; });
            gp::cgo::store_keep(x, x->next, head);
            head = x;
        }
        int64_t sum = 0;
        for (gp::cptr<Node> p = head; p != nullptr; p = p->next) { // a raw read per step
            sum += p->v;
        }
        CHECK_EQ(sum, int64_t(n) * (n - 1) / 2);
    } // head goes: the whole list is freed iteratively
    {
        gp::cptr<Node> head;
        for (int i = 0; i < n; i++) {
            head = gp::cgo::box_keep<Node>(
                [i](const gp::cptr<Node>& h) -> Node { return Node{i, h}; }, head);
        }
        CHECK_EQ(head->v, n - 1);
    }
    // 100K stores into a 1000-slot array member take no time worth the name (an index, not a scan).
    auto big = owned<Big>([] { return Big{}; });
    auto x = owned<Node>([] { return Node{1, nullptr}; });
    const auto t0 = std::chrono::steady_clock::now();
    for (int i = 0; i < 100000; i++) {
        gp::cgo::store_keep(big, big->slots[i % 1000], x);
    }
    const auto ms =
        std::chrono::duration_cast<std::chrono::milliseconds>(std::chrono::steady_clock::now() - t0)
            .count();
    CHECK(ms < 3000);
    CHECK_EQ(int64_t(x.gp_shared().use_count()), int64_t(1001));
}

void test_out_ptr() {
    gp::ptr<void> p;
    {
        gp::ptr<gp::ptr<void>> box = gp::new_<gp::ptr<void>>();
        alloc_out(gp::cgo::out_ptr(box));
        CHECK(*box != nullptr);
        CHECK_EQ(static_cast<char*>((*box).gp_get())[7], 'q');
        std::free((*box).gp_get());
        *box = gp::ptr<void>();
    }
    // A function that doesn't write leaves p's owner alone.
    gp::ptr<int64_t> x = gp::new_<int64_t>(3);
    gp::ptr<gp::ptr<void>> box = gp::new_<gp::ptr<void>>(gp::unsafe_pointer(x));
    auto noop = [](void**) {};
    noop(gp::cgo::out_ptr(box));
    CHECK(same_owner((*box).gp_shared(), x.gp_shared())); // (== compares the addresses, not owners)
    CHECK(!same_owner((*box).gp_shared(), gp::foreign_owner()));
    // The write-back happens when the statement ends.
    void* got = nullptr;
    auto take = [&](void** out) {
        got = *out;
        *out = nullptr;
    };
    take(gp::cgo::out_ptr(box));
    CHECK(got == x.gp_get() && *box == nullptr);
}

// --- strings.Builder, as Go ports it ---

// Builder is Go's strings.Builder: a []byte, and String() is unsafe.String over it.
struct Builder {
    gp::slice<uint8_t> buf;

    void write_string(const gp::string& s) {
        buf = gp::append_slice(std::move(buf), gp::slice<uint8_t>(gp::to_bytes(s)));
    }
    void write_byte(uint8_t c) { buf = gp::append(std::move(buf), c); }
    gp::string string() const {
        return gp::unsafe_string(gp::unsafe_slice_data(buf), gp::len(buf));
    }
};

gp::string build(int n) {
    Builder b; // gone when build returns: only the string keeps the array
    for (int i = 0; i < n; i++) {
        b.write_string("item "_gs);
        b.write_byte(uint8_t('0' + i % 10));
        b.write_byte(uint8_t('\n'));
    }
    return b.string();
}

void test_builder() {
    gp::string s = build(3);
    CHECK_EQ(str(s), std::string("item 0\nitem 1\nitem 2\n"));
    // Reads of the array that b freed in Go terms (ASan finds a view that owns nothing).
    gp::string big = build(2000);
    CHECK_EQ(gp::len(big), int64_t(2000 * 7));
    CHECK(big[0] == 'i' && big[2000 * 7 - 1] == '\n');
    // A growth after String() leaves the string's bytes alone, as in Go (append copies to a new
    // array when the capacity is full, and writes after the length otherwise).
    Builder b;
    b.write_string("abc"_gs);
    gp::string first = b.string();
    for (int i = 0; i < 100; i++) {
        b.write_byte('x');
    }
    CHECK_EQ(str(first), std::string("abc"));
    CHECK_EQ(gp::len(b.string()), int64_t(103));
    // The strings are values: a copy and a substring share the array, and the last one frees it.
    gp::string copy = big;
    gp::string tail = gp::sub(big, 2000 * 7 - 7);
    big = gp::string();
    CHECK_EQ(str(copy).substr(0, 6), std::string("item 0"));
    copy = gp::string();
    CHECK_EQ(str(tail), std::string("item 9\n"));
}

// --- the layout ---

void test_layout() {
    static_assert(sizeof(gp::ptr<void>) == 16);
    static_assert(
        std::is_same_v<decltype(gp::unsafe_pointer(std::declval<gp::ptr<int>>())), gp::ptr<void>>);
    // A named pointer type (gp::newtype over a gp::ptr) converts like the pointer.
    struct NamedP : gp::newtype<NamedP, gp::ptr<int64_t>> {
        using gp::newtype<NamedP, gp::ptr<int64_t>>::newtype;
    };
    NamedP np(gp::new_<int64_t>(4));
    gp::ptr<void> u = gp::unsafe_pointer(np);
    CHECK_EQ(*gp::unsafe_cast<int64_t>(u), int64_t(4));
}

void run_tests() {
    test_pointer();
    test_addr();
    test_add();
    test_slice_data();
    test_slice();
    test_slice_c();
    test_string_data();
    test_string();
    test_string_c();
    test_cgo_as();
    test_builder();
    test_cptr();
    test_out_ptr();
    test_out_cptr();
    test_box_keep();
    test_store_keep();
    test_read_back();
    test_side_keep();
    test_long_list();
    test_sizes();
    test_layout();
    if (failures > 0) {
        std::string msg =
            "FAIL: " + std::to_string(failures) + " of " + std::to_string(checks) + " checks\n";
        gp::os::write(gp::os::stderr_fd, msg.data(), msg.size());
        gp::os::exit(1);
    }
    gp::fmtshim::Printf("ok: %d checks\n"_gs, int64_t(checks));
}

// The cases that end the program (cxx_test.go's runUnsafe checks their output and exit code),
// from the program's argument.
const char* the_case = nullptr;

alignas(8) char bytes[16];

void run_case() {
    std::string c = the_case;
    if (c == "misaligned") {
        // (*int64)(p) of an odd address: Go's checkptr fatal error.
        gp::ptr<void> p = gp::cgo::as<gp::ptr<void>>(bytes + 1);
        gp::println(*gp::unsafe_cast<int64_t>(p));
    } else if (c == "misaligned-c") {
        // The same for a C pointer type.
        gp::ptr<void> p = gp::cgo::as<gp::ptr<void>>(bytes + 2);
        gp::println(*gp::cgo::as<int32_t*>(p));
    } else if (c == "slice-len") {
        gp::println(gp::len(gp::unsafe_slice(bytes, -1)));
    } else if (c == "store-keep-unowned") {
        // A pointer that a gocpp box owns, stored into C memory: nothing could keep it alive.
        Node cnode{1, nullptr};
        gp::cptr<Node> cp = &cnode;
        auto x = owned<Node>([] { return Node{2, nullptr}; });
        gp::cgo::store_keep(cp, cp->next, x);
        gp::println("not reached"_gs);
    } else if (c == "string-nil") {
        gp::println(gp::unsafe_string(gp::ptr<uint8_t>(), 1));
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
