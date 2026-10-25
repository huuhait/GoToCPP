// The unit tests of the GC engine's gp::gc::string and gp::gc::heap (phase-16 README §2.2 and
// §2.4; job 16.1c). runtime/cxx_test.go compiles this file with the runtime's .cpp files and
// with the GC engine's sources (rtfiles.GCSources), with g++ and clang++, at -O0, at -O2 and
// with ASan + UBSan, and runs it like test/unit.cpp: it prints "ok: N checks" and exits 0, or
// prints each failed check and exits 1.
#include <gocpp/gc/rt.hpp>

#include <cstdint>
#include <cstdio>
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

// check_string reports a string against its expected bytes.
void check_string(const char* what, gp::gc::string got, const char* want) {
    checks++;
    if (got.gp_view() != want) {
        failures++;
        std::fprintf(stderr, "%s: got %s, want %s\n", what, std::string(got.gp_view()).c_str(),
                     want);
    }
}

// The malloc backend of job 16.1c never frees, so the test keeps every allocation reachable:
// LeakSanitizer (the asan mode) must not report the test's own intentional leaks. The list is
// never destroyed (the same pattern as the runtime's own startup state): LeakSanitizer runs
// after the global destructors, and a destroyed vector would no longer be scanned. The bdwgc
// backend of job 16.1d collects the allocations, and this list then keeps nothing.
std::vector<const void*>& keep() {
    static auto* list = new std::vector<const void*>;
    return *list;
}

template <typename T>
T* keep_ptr(T* p) {
    keep().push_back(p);
    return p;
}

// keep_string keeps the bytes of a concatenation reachable, including the intermediate strings
// of a + b + c whose values the result replaces.
gp::gc::string keep_string(gp::gc::string s) {
    if (s.gp_data() != nullptr) {
        keep().push_back(s.gp_data());
    }
    return s;
}

} // namespace

int main() {
    using gp::gc::string;

    // Go's string header is a pointer and a length: 16 bytes (README §2.2).
    CHECK(sizeof(string) == 16);
    CHECK(alignof(string) == 8);

    // The zero value: a null pointer and a zero length.
    string zero;
    CHECK(zero.gp_data() == nullptr);
    CHECK(zero.gp_len() == 0);
    CHECK(gp::gc::len(zero) == 0);
    CHECK(zero == string());

    // A literal is static data: no copy and no allocation.
    const auto before = gp::gc::process_heap().stats();
    string hello = gp::gc::lit("hello");
    const auto after = gp::gc::process_heap().stats();
    CHECK(after.allocs == before.allocs);
    CHECK(after.bytes == before.bytes);
    CHECK(hello.gp_len() == 5);
    CHECK(gp::gc::len(hello) == 5);
    check_string("lit", hello, "hello");
    CHECK(hello.gp_data()[hello.gp_len()] == '\0'); // a literal has its NUL there

    // The comparisons Go has for strings: == and != by value, and < <= > >= in Go's string
    // order (the bytes lexicographically, the shorter first when one is a prefix).
    CHECK(gp::gc::lit("abc") == gp::gc::lit("abc"));
    CHECK(gp::gc::lit("abc") != gp::gc::lit("abd"));
    CHECK(gp::gc::lit("abc") != gp::gc::lit("ab"));
    CHECK(gp::gc::lit("") == string());
    CHECK(gp::gc::lit("") == gp::gc::lit(""));
    CHECK(gp::gc::lit("a") != string());
    CHECK(gp::gc::lit("abc") < gp::gc::lit("abd"));
    CHECK(gp::gc::lit("abc") <= gp::gc::lit("abc"));
    CHECK(gp::gc::lit("abc") < gp::gc::lit("abcd")); // a prefix is smaller
    CHECK(gp::gc::lit("abcd") > gp::gc::lit("abc"));
    CHECK(gp::gc::lit("abc") >= gp::gc::lit("abc"));
    CHECK(!(gp::gc::lit("abd") < gp::gc::lit("abc")));
    CHECK(gp::gc::compare(gp::gc::lit("abc"), gp::gc::lit("abd")) < 0);
    CHECK(gp::gc::compare(gp::gc::lit("abc"), gp::gc::lit("abc")) == 0);
    CHECK(gp::gc::compare(gp::gc::lit("abcd"), gp::gc::lit("abc")) > 0);

    // A concatenation owns its bytes in the GC heap: one allocation for each +, and the bytes
    // are NUL-terminated one past the length.
    const auto beforeConcat = gp::gc::process_heap().stats();
    string helloSpace = keep_string(gp::gc::lit("hello") + gp::gc::lit(" "));
    string helloWorld = keep_string(helloSpace + gp::gc::lit("world"));
    const auto afterConcat = gp::gc::process_heap().stats();
    check_string("concat", helloWorld, "hello world");
    CHECK(afterConcat.allocs == beforeConcat.allocs + 2);
    CHECK(afterConcat.bytes == beforeConcat.bytes + (6 + 1) + (11 + 1));
    CHECK(helloWorld.gp_data()[helloWorld.gp_len()] == '\0');
    check_string("concat empty left", keep_string(string() + gp::gc::lit("x")), "x");
    check_string("concat empty right", keep_string(gp::gc::lit("x") + string()), "x");
    CHECK(keep_string(string() + string()).gp_len() == 0);

    // s += t is Go's concatenation assignment: the variable gets a new string over new bytes.
    string plus = gp::gc::lit("ab");
    plus += gp::gc::lit("cd");
    plus = keep_string(plus);
    check_string("operator+=", plus, "abcd");
    plus += string();
    plus = keep_string(plus);
    check_string("operator+= empty", plus, "abcd");

    // A NUL inside the bytes is a byte of the string, as in Go.
    string withNul = gp::gc::lit("a\0b");
    CHECK(withNul.gp_len() == 3);
    CHECK(withNul.gp_view().size() == 3);

    // The spike backend of job 16.1c, behind the interface that bdwgc fills in job 16.1d.
    CHECK(std::string(gp::gc::process_heap().name()) == "malloc");

    // alloc and alloc_atomic return zeroed bytes (Go's zero value rule).
    auto* scanned = static_cast<unsigned char*>(keep_ptr(gp::gc::process_heap().alloc(8)));
    auto* atomic = static_cast<unsigned char*>(keep_ptr(gp::gc::process_heap().alloc_atomic(8)));
    for (int i = 0; i < 8; i++) {
        CHECK(scanned[i] == 0);
        CHECK(atomic[i] == 0);
    }

    // new_ and new_atomic_ construct in the GC heap.
    struct pair {
        std::int64_t a;
        std::int64_t b;
    };
    auto* p = keep_ptr(gp::gc::new_<pair>(pair{1, 2}));
    CHECK(p->a == 1 && p->b == 2);
    auto* z = keep_ptr(gp::gc::new_atomic_<std::int64_t>());
    CHECK(*z == 0);

    // A root keeps a pointer, can be changed, and is released. The malloc backend never frees,
    // so this test can check the root API only, not that a root keeps an object alive: the
    // liveness proof needs a collector and comes with the bdwgc backend and its tests in job
    // 16.1d.
    std::int64_t value = 7;
    void* handle = gp::gc::process_heap().make_root(&value);
    CHECK(gp::gc::process_heap().root_value(handle) == &value);
    gp::gc::process_heap().set_root(handle, nullptr);
    CHECK(gp::gc::process_heap().root_value(handle) == nullptr);
    gp::gc::process_heap().drop_root(handle);
    {
        gp::gc::root r(&value);
        CHECK(r.get() == &value);
        r.set(nullptr);
        CHECK(r.get() == nullptr);
    }
    {
        gp::gc::root a(&value);
        gp::gc::root b(std::move(a));
        CHECK(b.get() == &value);
    }

    // A blocking section runs its function.
    int calls = 0;
    gp::gc::blocking([&calls] { calls++; });
    CHECK(calls == 1);

    // collect counts, and thread registration is a no-op in the malloc backend.
    const auto beforeCollect = gp::gc::process_heap().stats();
    gp::gc::process_heap().collect();
    const auto afterCollect = gp::gc::process_heap().stats();
    CHECK(afterCollect.collections == beforeCollect.collections + 1);
    gp::gc::process_heap().enter_thread();
    gp::gc::process_heap().leave_thread();

    if (failures != 0) {
        std::fprintf(stderr, "%d of %d checks failed\n", failures, checks);
        return 1;
    }
    std::printf("ok: %d checks\n", checks);
    return 0;
}
