// The unit tests of type descriptors (gocpp/type.hpp, Phase 05 step 5.1): the predeclared
// descriptors (int and int64 are different objects), named types and their method tables,
// the interned composite descriptors (one pointer per Go type, also from many threads) with
// Go's names for them (the texts below are what `%T` prints in Go 1.25, checked with a real
// program), channel directions, foreign (C/C++) descriptors, opaque descriptors, the hooks
// for == and hash, and the order of method tables. runtime/cxx_test.go compiles this file with
// the runtime's .cpp files, with g++ and clang++, at -O0, at -O2 and with ASan + UBSan
// (LeakSanitizer included), and runs it like test/unit.cpp. It prints "ok: N checks" and
// exits 0, or prints each failed check and exits 1.
#include <gocpp/rt.hpp>
#include "threads_ended.hpp"
#include <gocpp/os.hpp>

#include <cmath>
#include <cstdint>
#include <iterator>
#include <memory>
#include <set>
#include <string>
#include <string_view>
#include <thread>
#include <type_traits>
#include <utility>
#include <vector>

namespace {

int checks = 0;
int failures = 0;

void report(bool ok, const char* file, int line, const std::string& what) {
    checks++;
    if (!ok) {
        failures++;
        std::string msg = std::string(file) + ":" + std::to_string(line) + ": " + what + "\n";
        gp::os::write(gp::os::stderr_fd, msg.data(), msg.size());
    }
}

#define CHECK(...) report((__VA_ARGS__), __FILE__, __LINE__, "CHECK(" #__VA_ARGS__ ") failed")

// CHECK_NAME checks the Go name of a descriptor.
#define CHECK_NAME(td, want)                                                                       \
    do {                                                                                           \
        std::string gp_got = (td)->gp_name;                                                        \
        std::string gp_want = (want);                                                              \
        report(gp_got == gp_want, __FILE__, __LINE__,                                              \
               "the name of " #td " is \"" + gp_got + "\", want \"" + gp_want + "\"");             \
    } while (0)

// --- C++ types for the tests ---

struct Rect {
    double W = 0;
    double H = 0;
    bool operator==(const Rect&) const = default;
    std::size_t gp_hash() const { return gp::hash_fields(W, H); }
};

struct Empty {
    bool operator==(const Empty&) const = default;
    std::size_t gp_hash() const { return gp::hash_fields(); }
};

struct NoEq {
    int64_t x = 0;
};

struct Celsius : gp::newtype<Celsius, double> {
    using gp::newtype<Celsius, double>::newtype;
};

struct IDs : gp::newtype<IDs, gp::slice<int64_t>> {
    using gp::newtype<IDs, gp::slice<int64_t>>::newtype;
};

struct Table : gp::newtype<Table, gp::map<gp::string, int64_t>> {
    using gp::newtype<Table, gp::map<gp::string, int64_t>>::newtype;
};

struct Handler : gp::newtype<Handler, gp::func<void()>> {
    using gp::newtype<Handler, gp::func<void()>>::newtype;
};

// An interface struct with nothing in it (the shape of generated code).
struct Shape : gp::iface {
    using gp::iface::iface;
};

// fake_chan stands for gp::chan<T> (Phase 08): one C++ type for the three directions, equal
// when it is the same channel.
template <typename T>
struct fake_chan {
    std::shared_ptr<T> gp_state;
    bool operator==(const fake_chan& o) const { return gp_state == o.gp_state; }
    std::size_t gp_hash() const { return std::hash<std::shared_ptr<T>>{}(gp_state); }
};

// C++ classes for foreign descriptors: with == and std::hash, with == only, with neither; an
// enum.
struct Point {
    int x = 0;
    int y = 0;
    bool operator==(const Point&) const = default;
};
struct Plain {
    int x = 0;
};
struct Hashed {
    int x = 0;
    bool operator==(const Hashed&) const = default;
};
enum Color { Red, Green };

} // namespace

template <>
struct std::hash<Point> {
    std::size_t operator()(const Point& p) const noexcept {
        return static_cast<std::size_t>(p.x * 31 + p.y);
    }
};

namespace {

// Thunks of the shape that the emitter writes, which these tests never call.
double thunk_area(const gp::iface&) {
    return 0;
}
void thunk_scale(const gp::iface&, double) {}

template <typename T>
bool td_equal(const gp::type_desc* td, const T& a, const T& b) {
    return td->gp_equal != nullptr && td->gp_equal(&a, &b);
}

template <typename T>
std::size_t td_hash(const gp::type_desc* td, const T& v) {
    return td->gp_hash(&v);
}

// ---

void test_predeclared() {
    using namespace gp;
    struct row {
        const type_desc* td;
        const char* name;
        kind k;
        std::size_t size;
    };
    const row rows[] = {
        {&td_bool, "bool", kind::Bool, 1},
        {&td_int, "int", kind::Int, 8},
        {&td_int8, "int8", kind::Int8, 1},
        {&td_int16, "int16", kind::Int16, 2},
        {&td_int32, "int32", kind::Int32, 4},
        {&td_int64, "int64", kind::Int64, 8},
        {&td_uint, "uint", kind::Uint, 8},
        {&td_uint8, "uint8", kind::Uint8, 1},
        {&td_uint16, "uint16", kind::Uint16, 2},
        {&td_uint32, "uint32", kind::Uint32, 4},
        {&td_uint64, "uint64", kind::Uint64, 8},
        {&td_uintptr, "uintptr", kind::Uintptr, 8},
        {&td_float32, "float32", kind::Float32, 4},
        {&td_float64, "float64", kind::Float64, 8},
        {&td_complex64, "complex64", kind::Complex64, 8},
        {&td_complex128, "complex128", kind::Complex128, 16},
        {&td_string, "string", kind::String, sizeof(gp::string)},
        {&td_unsafe_pointer, "unsafe.Pointer", kind::UnsafePointer, sizeof(gp::ptr<void>)},
    };
    std::set<const type_desc*> seen;
    for (const row& r : rows) {
        CHECK_NAME(r.td, r.name);
        CHECK(r.td->gp_kind == r.k);
        CHECK(r.td->gp_size == r.size);
        CHECK(r.td->gp_named);
        CHECK(r.td->gp_pkg == nullptr);
        CHECK(r.td->gp_equal != nullptr && r.td->gp_hash != nullptr);
        CHECK(r.td->gp_nmethods == 0 && r.td->gp_npmethods == 0);
        seen.insert(r.td);
    }
    // Every one is its own object: int and int64 share int64_t, and uint, uint64 and uintptr
    // share uint64_t, but they are different Go types.
    CHECK(seen.size() == std::size(rows));
    CHECK(&td_int != &td_int64);
    CHECK(&td_uint != &td_uint64 && &td_uint != &td_uintptr && &td_uint64 != &td_uintptr);
    CHECK(td_int.gp_size == td_int64.gp_size && td_int.gp_align == td_int64.gp_align);

    // The hooks, on the values of the C++ type.
    int64_t a = 5, b = 5, c = 6;
    CHECK(td_equal(&td_int, a, b) && !td_equal(&td_int, a, c));
    CHECK(td_hash(&td_int, a) == td_hash(&td_int, b));
    CHECK(td_hash(&td_int, a) == td_hash(&td_int64, a)); // the same C++ type, the same hash
    CHECK(td_equal(&td_int8, int8_t(-1), int8_t(-1)));
    CHECK(td_equal(&td_uint16, uint16_t(65535), uint16_t(65535)));
    double nan = std::nan("");
    CHECK(!td_equal(&td_float64, nan, nan));
    CHECK(td_equal(&td_float64, 0.0, -0.0));
    CHECK(td_hash(&td_float64, 0.0) == td_hash(&td_float64, -0.0));
    CHECK(td_equal(&td_float32, 1.5f, 1.5f));
    CHECK(td_equal(&td_string, "abc"_gs, "abc"_gs));
    CHECK(!td_equal(&td_string, "abc"_gs, "abd"_gs));
    CHECK(td_hash(&td_string, "abc"_gs) == td_hash(&td_string, "abc"_gs));
    CHECK(td_equal(&td_bool, true, true) && !td_equal(&td_bool, true, false));
    CHECK(td_equal(&td_complex128, gp::complex128(1, 2), gp::complex128(1, 2)));
    CHECK(!td_equal(&td_complex128, gp::complex128(1, 2), gp::complex128(1, 3)));
    int x = 0, y = 0;
    gp::ptr<void> px(gp::static_owner(), &x), px2(gp::static_owner(), &x),
        py(gp::static_owner(), &y);
    // The hooks of a pointer take the target addresses (what an interface holds: the data of
    // test/iface_test.cpp goes through an interface), not gp::ptr objects.
    CHECK(td_unsafe_pointer.gp_equal(px.gp_get(), px2.gp_get()));
    CHECK(!td_unsafe_pointer.gp_equal(px.gp_get(), py.gp_get()));
    CHECK(td_unsafe_pointer.gp_equal(nullptr, nullptr));
    CHECK(td_unsafe_pointer.gp_hash(px.gp_get()) == td_unsafe_pointer.gp_hash(px2.gp_get()));
    CHECK(td_unsafe_pointer.gp_hash(px.gp_get()) == gp::hash_of(px));

    // interface {} is one object, whichever way it is asked for.
    CHECK_NAME(&td_any, "interface {}");
    CHECK(td_any.gp_kind == kind::Interface && td_any.gp_size == 32 && td_any.gp_nimethods == 0);
    CHECK(td_interface({}) == &td_any);
    CHECK(td_any.gp_equal == nullptr && td_any.gp_hash == nullptr); // interface ==: step 5.6
    CHECK(!td_any.gp_named);
}

// The Go names of the composite types.
void test_names() {
    using namespace gp;
    const type_desc* tdi = &td_int;
    const type_desc* tds = &td_string;
    using C = fake_chan<int64_t>;
    using F = gp::func<void()>;

    // Channel directions: three Go types, one C++ type.
    const type_desc* cb = td_chan<C>(chan_dir::both, tdi);
    const type_desc* cr = td_chan<C>(chan_dir::recv, tdi);
    const type_desc* cs = td_chan<C>(chan_dir::send, tdi);
    CHECK_NAME(cb, "chan int");
    CHECK_NAME(cr, "<-chan int");
    CHECK_NAME(cs, "chan<- int");
    CHECK(cb->gp_kind == kind::Chan && cb->gp_dir == chan_dir::both && cb->gp_elem == tdi);
    CHECK(cr->gp_dir == chan_dir::recv && cs->gp_dir == chan_dir::send);
    CHECK(cb != cr && cb != cs && cr != cs);
    // "<-" binds to the leftmost chan it can, so a bidirectional channel of a receive-only one
    // needs parentheses.
    CHECK_NAME(td_chan<C>(chan_dir::both, cr), "chan (<-chan int)");
    CHECK_NAME(td_chan<C>(chan_dir::send, cr), "chan<- <-chan int");
    CHECK_NAME(td_chan<C>(chan_dir::recv, cb), "<-chan chan int");
    CHECK_NAME(td_chan<C>(chan_dir::recv, cr), "<-chan <-chan int");
    CHECK_NAME(td_chan<C>(chan_dir::both, td_chan<C>(chan_dir::send, tdi)), "chan chan<- int");

    // Funcs.
    static const imethod error_methods[] = {imethod("Error", nullptr, "func() string")};
    static const type_desc td_error =
        named<Shape>("error", nullptr, shape_interface(error_methods));
    const type_desc* sl_string = td_slice<gp::string>(tds);
    const type_desc* f_variadic = td_func<F>({tdi, sl_string}, {&td_bool, &td_error}, true);
    CHECK_NAME(f_variadic, "func(int, ...string) (bool, error)");
    CHECK(f_variadic->gp_kind == kind::Func && f_variadic->gp_variadic && f_variadic->gp_nin == 2 &&
          f_variadic->gp_nout == 2 && f_variadic->gp_in[1] == sl_string);
    CHECK_NAME(td_func<F>({tdi, sl_string}, {&td_bool, &td_error}, false),
               "func(int, []string) (bool, error)");
    const type_desc* f1 = td_func<F>({}, {});
    const type_desc* f2 = td_func<F>({tdi}, {tds});
    CHECK_NAME(f1, "func()");
    CHECK_NAME(td_func<F>({}, {tdi}), "func() int");
    CHECK_NAME(f2, "func(int) string");
    CHECK_NAME(td_func<F>({tdi, tdi}, {tdi, tdi}), "func(int, int) (int, int)");
    CHECK_NAME(td_func<F>({td_func<F>({f2}, {f1})}, {f1}),
               "func(func(func(int) string) func()) func()");
    const type_desc* any_slice = td_slice<gp::any>(&td_any);
    CHECK_NAME(any_slice, "[]interface {}");
    CHECK_NAME(td_func<F>({cs, cr, any_slice}, {}, true),
               "func(chan<- int, <-chan int, ...interface {})");

    // map[string][]*[3]map[int]struct {}
    const type_desc* empty_struct = td_struct<Empty>({});
    CHECK_NAME(empty_struct, "struct {}");
    const type_desc* m_int = td_map<int64_t, Empty>(tdi, empty_struct);
    const type_desc* a3 = td_array<gp::map<int64_t, Empty>, 3>(m_int);
    const type_desc* pa3 = td_ptr<gp::array<gp::map<int64_t, Empty>, 3>>(a3);
    const type_desc* spa3 = td_slice<gp::ptr<gp::array<gp::map<int64_t, Empty>, 3>>>(pa3);
    CHECK_NAME(spa3, "[]*[3]map[int]struct {}");
    CHECK_NAME((td_map<gp::string, int>(tds, spa3)), "map[string][]*[3]map[int]struct {}");
    CHECK_NAME((td_array<Shape, 2>(&td_error)), "[2]error");
    CHECK_NAME((td_array<int, 0>(tdi)), "[0]int");
    CHECK_NAME((td_array<int, 1000000>(tdi)), "[1000000]int");

    // Structs: embedded fields have their type's name, tags are quoted as Go quotes them,
    // unexported names show no package.
    static const type_desc td_R = named<Rect>("main.R", "main", shape_struct(nullptr, 0));
    static const type_desc td_C = named<Rect>("main.C", "main", shape_struct(nullptr, 0));
    const type_desc* p_C = td_ptr<Rect>(&td_C);
    CHECK_NAME(p_C, "*main.C");
    const type_desc* big =
        td_struct<Rect>({field_desc("X", nullptr, tdi, "json:\"x\""), field_desc("y", "main", tds),
                         field_desc("R", nullptr, &td_R, nullptr, true),
                         field_desc("C", nullptr, p_C, nullptr, true),
                         field_desc("Z", nullptr, &td_float64, "tag2"),
                         field_desc("W", nullptr, &td_float64, "tag2")});
    CHECK_NAME(big, "struct { X int \"json:\\\"x\\\"\"; y string; main.R; *main.C; Z float64 "
                    "\"tag2\"; W float64 \"tag2\" }");
    CHECK(big->gp_kind == kind::Struct && big->gp_nfields == 6 && big->gp_fields[2].gp_embedded &&
          !big->gp_fields[0].gp_embedded);
    CHECK_NAME(td_struct<Rect>({field_desc("a", "main", tdi), field_desc("b", "main", tdi)}),
               "struct { a int; b int }");
    CHECK_NAME(td_struct<Rect>({field_desc("A", nullptr, tdi, "a\"b\n\\")}),
               "struct { A int \"a\\\"b\\n\\\\\" }");
    CHECK_NAME(td_struct<Rect>({field_desc("F", nullptr, td_func<F>({}, {tdi})),
                                field_desc("G", nullptr, td_ptr<int64_t>(tdi))}),
               "struct { F func() int; G *int }");

    // Unnamed interfaces: the methods in their order, an unexported name with its package's
    // name; named ones carry their own name.
    CHECK_NAME(td_interface({imethod("m", "main", "func() int")}), "interface { main.m() int }");
    CHECK_NAME(td_interface({imethod("Area", nullptr, "func() float64"),
                             imethod("String", nullptr, "func() string")}),
               "interface { Area() float64; String() string }");
    CHECK_NAME(
        td_interface({imethod("N", nullptr, "func()"), imethod("m", "x/sub/pk", "func()", "pkn")}),
        "interface { N(); pkn.m() }");
    // Without the package's name: the last element of its path.
    CHECK_NAME(td_interface({imethod("N", nullptr, "func()"), imethod("m2", "x/sub/pk", "func()")}),
               "interface { N(); pk.m2() }");
    CHECK_NAME(td_struct<Rect>({field_desc("x", "main",
                                           td_interface({imethod("M", nullptr, "func()"),
                                                         imethod("n", "main", "func()")}))}),
               "struct { x interface { M(); main.n() } }");
    // The emitter may give the exact text (Go's has package names where the signature text has
    // paths).
    CHECK_NAME(td_interface({imethod("Get", nullptr, "func() net/url.Values")},
                            "interface { Get() url.Values }"),
               "interface { Get() url.Values }");
    CHECK_NAME(td_interface({imethod("Put", nullptr, "func() net/url.Values")}),
               "interface { Put() net/url.Values }");
}

void test_interning() {
    using namespace gp;
    const type_desc* tdi = &td_int;
    const type_desc* tds = &td_string;

    // The same Go type, the same pointer.
    const type_desc* s_int = td_slice<int64_t>(tdi);
    CHECK(s_int == td_slice<int64_t>(tdi));
    CHECK_NAME(s_int, "[]int");
    CHECK(s_int->gp_kind == kind::Slice && s_int->gp_elem == tdi &&
          s_int->gp_size == sizeof(gp::slice<int64_t>));
    CHECK(!s_int->gp_named);
    // []int and []int64 share one C++ type, and are two Go types.
    const type_desc* s_int64 = td_slice<int64_t>(&td_int64);
    CHECK(s_int != s_int64);
    CHECK_NAME(s_int64, "[]int64");
    CHECK(td_ptr<int64_t>(tdi) != td_ptr<int64_t>(&td_int64));
    CHECK_NAME(td_ptr<int64_t>(tdi), "*int");
    CHECK_NAME(td_ptr<int64_t>(&td_int64), "*int64");
    CHECK(td_ptr<int64_t>(tdi) == td_ptr<int64_t>(tdi));
    CHECK(td_ptr<int64_t>(tdi)->gp_size == sizeof(gp::ptr<int64_t>));
    CHECK(td_array<int64_t, 3>(tdi) == td_array<int64_t, 3>(tdi));
    CHECK(td_array<int64_t, 3>(tdi) != td_array<int64_t, 4>(tdi));
    CHECK(td_array<int64_t, 3>(tdi) != td_array<int64_t, 3>(&td_int64));
    CHECK(td_array<int64_t, 3>(tdi)->gp_len == 3 && td_array<int64_t, 3>(tdi)->gp_size == 24);
    CHECK(td_map<gp::string, int64_t>(tds, tdi) == td_map<gp::string, int64_t>(tds, tdi));
    CHECK(td_map<gp::string, int64_t>(tds, tdi) != td_map<gp::string, int64_t>(tds, &td_int64));
    CHECK(td_map<gp::string, int64_t>(tds, tdi) != td_map<int64_t, gp::string>(tdi, tds));
    // The key is part of the type: the same element with another key is another type.
    CHECK(td_map<int64_t, gp::string>(tdi, tds) != td_map<int64_t, gp::string>(&td_int64, tds));
    CHECK(td_map<int64_t, gp::string>(tdi, tds) != td_map<int32_t, gp::string>(&td_int32, tds));
    CHECK(td_map<int64_t, gp::string>(tdi, tds) == td_map<int64_t, gp::string>(tdi, tds));
    CHECK(td_map<int64_t, gp::string>(&td_int64, tds) ==
          td_map<int64_t, gp::string>(&td_int64, tds));
    CHECK(td_map<gp::string, int64_t>(tds, tdi)->gp_key == tds);
    CHECK(td_slice<gp::slice<int64_t>>(s_int) ==
          td_slice<gp::slice<int64_t>>(td_slice<int64_t>(tdi)));
    CHECK(td_slice<gp::slice<int64_t>>(s_int) != s_int);

    // A new Go type adds one descriptor to the table; a lookup adds none.
    const std::size_t before = detail::interned_count();
    const type_desc* fresh = td_array<uint8_t, 7777>(&td_uint8);
    CHECK(detail::interned_count() == before + 1);
    CHECK(td_array<uint8_t, 7777>(&td_uint8) == fresh);
    CHECK(td_array<uint8_t, 7777>(&td_uint8) == fresh);
    CHECK(detail::interned_count() == before + 1);

    // Channels: the direction is part of the type.
    using C = fake_chan<int64_t>;
    CHECK(td_chan<C>(chan_dir::both, tdi) == td_chan<C>(chan_dir::both, tdi));
    CHECK(td_chan<C>(chan_dir::recv, tdi) == td_chan<C>(chan_dir::recv, tdi));
    CHECK(td_chan<C>(chan_dir::send, tdi) == td_chan<C>(chan_dir::send, tdi));
    CHECK(td_chan<C>(chan_dir::recv, tdi) != td_chan<C>(chan_dir::send, tdi));
    CHECK(td_chan<C>(chan_dir::both, tdi) != td_chan<C>(chan_dir::both, &td_int64));
    CHECK(td_chan<C>(chan_dir::both, tdi)->gp_size == sizeof(C));
    // Channels compare by identity: the hooks come from the C++ type.
    auto ch1 = C{std::make_shared<int64_t>(1)};
    auto ch2 = C{std::make_shared<int64_t>(1)};
    CHECK(td_equal(td_chan<C>(chan_dir::recv, tdi), ch1, ch1));
    CHECK(!td_equal(td_chan<C>(chan_dir::recv, tdi), ch1, ch2));
    CHECK(td_hash(td_chan<C>(chan_dir::recv, tdi), ch1) ==
          td_hash(td_chan<C>(chan_dir::send, tdi), ch1));

    // Funcs: the parameters, the results and variadic are all part of the type.
    using F = gp::func<void()>;
    const type_desc* sl = td_slice<int64_t>(tdi);
    CHECK(td_func<F>({tdi}, {tds}) == td_func<F>({tdi}, {tds}));
    CHECK(td_func<F>({tdi}, {tds}) != td_func<F>({tds}, {tdi}));
    CHECK(td_func<F>({tdi}, {}) != td_func<F>({}, {tdi}));
    CHECK(td_func<F>({tdi, tds}, {}) != td_func<F>({tds, tdi}, {}));
    CHECK(td_func<F>({tdi}, {tdi}) != td_func<F>({tdi}, {tdi, tdi}));
    CHECK(td_func<F>({sl}, {}, true) != td_func<F>({sl}, {}, false));
    CHECK(td_func<F>({sl}, {}, true) == td_func<F>({sl}, {}, true));
    CHECK(td_func<F>({}, {}) == td_func<F>({}, {}));
    CHECK(td_func<F>({tdi}, {})->gp_equal == nullptr && td_func<F>({tdi}, {})->gp_hash == nullptr);

    // Structs: every part of a field is part of the type.
    auto st = [&](const char* name, const char* pkg, const type_desc* t, const char* tag,
                  bool emb) { return td_struct<Rect>({field_desc(name, pkg, t, tag, emb)}); };
    const type_desc* base = st("X", nullptr, tdi, nullptr, false);
    CHECK(base == st("X", nullptr, tdi, nullptr, false));
    CHECK(base != st("Y", nullptr, tdi, nullptr, false));
    CHECK(base != st("X", nullptr, &td_int64, nullptr, false));
    CHECK(base != st("X", nullptr, tdi, "tag", false));
    CHECK(base != st("X", nullptr, tdi, nullptr, true));
    // The same name in two packages (unexported) is two types.
    CHECK(st("x", "a", tdi, nullptr, false) != st("x", "b", tdi, nullptr, false));
    CHECK(st("x", "a", tdi, nullptr, false) == st("x", "a", tdi, nullptr, false));
    CHECK(td_struct<Rect>({field_desc("X", nullptr, tdi), field_desc("Y", nullptr, tdi)}) ==
          td_struct<Rect>({field_desc("X", nullptr, tdi), field_desc("Y", nullptr, tdi)}));
    CHECK(td_struct<Rect>({field_desc("X", nullptr, tdi), field_desc("Y", nullptr, tdi)}) !=
          td_struct<Rect>({field_desc("Y", nullptr, tdi), field_desc("X", nullptr, tdi)}));
    // The name's text is by value: the same tag text from another array is the same type.
    std::string tag1 = "json:\"a\"";
    std::string tag2 = "json:\"a\"";
    CHECK(st("T", nullptr, tdi, tag1.c_str(), false) == st("T", nullptr, tdi, tag2.c_str(), false));
    CHECK(base->gp_equal != nullptr && base->gp_hash != nullptr);

    // Interfaces: the methods (name, package, signature text) are the type; a named interface
    // type is a type of its own.
    const type_desc* i1 =
        td_interface({imethod("A", nullptr, "func() int"), imethod("B", nullptr, "func()")});
    CHECK(i1 ==
          td_interface({imethod("A", nullptr, "func() int"), imethod("B", nullptr, "func()")}));
    CHECK(i1 !=
          td_interface({imethod("A", nullptr, "func() int64"), imethod("B", nullptr, "func()")}));
    CHECK(i1 != td_interface({imethod("A", nullptr, "func() int")}));
    CHECK(td_interface({imethod("a", "p", "func()")}) !=
          td_interface({imethod("a", "q", "func()")}));
    CHECK(td_interface({imethod("a", "p", "func()")}) ==
          td_interface({imethod("a", "p", "func()")}));
    CHECK(i1->gp_kind == kind::Interface && i1->gp_nimethods == 2 && i1->gp_size == 32);
    CHECK(i1->gp_equal == nullptr && i1->gp_hash == nullptr);
    static const imethod named_methods[] = {imethod("A", nullptr, "func() int"),
                                            imethod("B", nullptr, "func()")};
    static const type_desc td_named_iface =
        named<Shape>("main.Both", "main", shape_interface(named_methods));
    CHECK(&td_named_iface != i1);
    CHECK(td_named_iface.gp_named && td_named_iface.gp_size == 32 &&
          td_named_iface.gp_nimethods == 2);
    CHECK(td_named_iface.gp_equal == nullptr);
    // A slice of the named one and a slice of the unnamed one are different types.
    CHECK(td_slice<Shape>(&td_named_iface) != td_slice<Shape>(i1));

    // The hooks of composite types come from the C++ type: pointers by address, arrays
    // element by element, slices, maps and funcs not at all.
    gp::ptr<int64_t> a = gp::new_<int64_t>(1), b = a, c = gp::new_<int64_t>(1);
    const type_desc* pi = td_ptr<int64_t>(tdi);
    CHECK(pi->gp_equal(a.gp_get(), b.gp_get()) && !pi->gp_equal(a.gp_get(), c.gp_get()));
    CHECK(pi->gp_equal(nullptr, nullptr) && !pi->gp_equal(a.gp_get(), nullptr));
    CHECK(pi->gp_hash(a.gp_get()) == pi->gp_hash(b.gp_get()));
    CHECK(pi->gp_hash(a.gp_get()) == gp::hash_of(a)); // the hash of the pointer, as a map key
    const type_desc* ai = td_array<int64_t, 2>(tdi);
    gp::array<int64_t, 2> x{1, 2}, y{1, 2}, z{1, 3};
    CHECK(td_equal(ai, x, y) && !td_equal(ai, x, z));
    CHECK(td_hash(ai, x) == td_hash(ai, y));
    CHECK(s_int->gp_equal == nullptr && s_int->gp_hash == nullptr);
    CHECK(td_map<gp::string, int64_t>(tds, tdi)->gp_equal == nullptr);
    // An array of slices is not comparable either (Go: invalid map key, == not defined).
    CHECK(td_array<gp::slice<int64_t>, 2>(s_int)->gp_equal == nullptr);
}

// The descriptors of named types, with their method tables.
void test_named() {
    using namespace gp;
    // constinit: a method table is constant-initialized (method_of is a constant expression), so
    // nothing of it is made at program start (a host's C constructor may call into a library
    // before its static initialization).
    static constinit const method vm[] = {
        method_of<double(), &thunk_area>("Area", nullptr, "func() float64")};
    static constinit const method pm[] = {
        method_of<double(), &thunk_area>("Area", nullptr, "func() float64"),
        method_of<void(double), &thunk_scale>("Scale", nullptr, "func(float64)")};
    static const field_desc fields[] = {field_desc("W", nullptr, &td_float64),
                                        field_desc("H", nullptr, &td_float64)};
    static const type_desc td_Rect =
        named<Rect>("main.Rect", "main", shape_struct(fields), methods_of(vm), methods_of(pm));
    CHECK_NAME(&td_Rect, "main.Rect");
    CHECK(td_Rect.gp_kind == kind::Struct && td_Rect.gp_named);
    CHECK(std::string(td_Rect.gp_pkg) == "main");
    CHECK(td_Rect.gp_size == sizeof(Rect) && td_Rect.gp_align == alignof(Rect));
    CHECK(td_Rect.gp_nfields == 2 && td_Rect.gp_fields == fields);
    // The method set of Rect is the value methods, the one of *Rect all of them.
    CHECK(td_Rect.gp_methods == vm && td_Rect.gp_nmethods == 1);
    CHECK(td_Rect.gp_pmethods == pm && td_Rect.gp_npmethods == 2);
    const type_desc* prect = td_ptr<Rect>(&td_Rect);
    CHECK_NAME(prect, "*main.Rect");
    CHECK(prect == td_ptr<Rect>(&td_Rect));
    CHECK(prect->gp_kind == kind::Pointer && prect->gp_elem == &td_Rect && !prect->gp_named);
    CHECK(prect->gp_methods == pm && prect->gp_nmethods == 2);
    CHECK(prect->gp_size == sizeof(gp::ptr<Rect>));
    // **Rect has no methods, and a pointer to a type without methods none.
    CHECK(td_ptr<gp::ptr<Rect>>(prect)->gp_nmethods == 0);
    CHECK(td_ptr<int64_t>(&td_int)->gp_nmethods == 0);
    CHECK(methods_sorted(pm, 2));
    // The thunks are stored as they are.
    CHECK(pm[0].gp_fn() == reinterpret_cast<gp::fn_ptr>(&thunk_area));
    CHECK(pm[1].gp_fn() == reinterpret_cast<gp::fn_ptr>(&thunk_scale));
    CHECK(std::string(pm[1].gp_sig) == "func(float64)");
    // == and hash from the C++ type.
    CHECK(td_equal(&td_Rect, Rect{1, 2}, Rect{1, 2}) &&
          !td_equal(&td_Rect, Rect{1, 2}, Rect{1, 3}));
    CHECK(td_hash(&td_Rect, Rect{1, 2}) == td_hash(&td_Rect, Rect{1, 2}));
    // A struct without == (not comparable in Go) has no hooks.
    static const type_desc td_NoEq = named<NoEq>("main.NoEq", "main", shape_struct(nullptr, 0));
    CHECK(td_NoEq.gp_equal == nullptr && td_NoEq.gp_hash == nullptr);
    CHECK(td_NoEq.gp_nfields == 0);

    // Named types over other types: the hooks follow Go's comparability of the kind.
    static const type_desc td_Celsius =
        named<Celsius>("main.Celsius", "main", shape_basic(kind::Float64));
    CHECK(td_Celsius.gp_kind == kind::Float64 && td_Celsius.gp_named);
    CHECK(td_equal(&td_Celsius, Celsius(1.5), Celsius(1.5)) &&
          !td_equal(&td_Celsius, Celsius(1.5), Celsius(2)));
    CHECK(td_hash(&td_Celsius, Celsius(1.5)) == td_hash(&td_Celsius, Celsius(1.5)));
    static const type_desc td_IDs = named<IDs>("main.IDs", "main", shape_slice(&td_int));
    CHECK(td_IDs.gp_kind == kind::Slice && td_IDs.gp_elem == &td_int);
    CHECK(td_IDs.gp_equal == nullptr &&
          td_IDs.gp_hash == nullptr); // newtype has gp_hash, still not comparable
    static const type_desc td_Table =
        named<Table>("main.Table", "main", shape_map(&td_string, &td_int));
    CHECK(td_Table.gp_equal == nullptr && td_Table.gp_hash == nullptr);
    static const type_desc td_Handler =
        named<Handler>("main.Handler", "main", shape_func(nullptr, 0, nullptr, 0, false));
    CHECK(td_Handler.gp_equal == nullptr && td_Handler.gp_hash == nullptr);
    // The descriptor of a slice of a named type: the name is built from the part's.
    CHECK_NAME(td_slice<Celsius>(&td_Celsius), "[]main.Celsius");
    CHECK_NAME(td_ptr<Celsius>(&td_Celsius), "*main.Celsius");
    CHECK_NAME((td_map<gp::string, Celsius>(&td_string, &td_Celsius)), "map[string]main.Celsius");

    // Named types over arrays and channels.
    static const type_desc td_Grid =
        named<gp::array<int64_t, 3>>("main.Grid", "main", shape_array(3, &td_int));
    CHECK(td_Grid.gp_kind == kind::Array && td_Grid.gp_len == 3 && td_Grid.gp_equal != nullptr);
    using C = fake_chan<int64_t>;
    static const type_desc td_Ch = named<C>("main.Ch", "main", shape_chan(chan_dir::recv, &td_int));
    CHECK(td_Ch.gp_kind == kind::Chan && td_Ch.gp_dir == chan_dir::recv &&
          td_Ch.gp_equal != nullptr);
    CHECK(td_Ch.gp_size == sizeof(C));
    // The named type and the type it is over are different types.
    CHECK(&td_Ch != td_chan<C>(chan_dir::recv, &td_int));
}

void test_method_order() {
    using namespace gp;
    // Exported names first, by name, in byte order; then the unexported ones, by name and by
    // package path.
    const method ok[] = {
        method("A", nullptr, "func()", nullptr), method("B", nullptr, "func()", nullptr),
        method("Z", nullptr, "func()", nullptr), method("a", "p", "func()", nullptr),
        method("a", "q", "func()", nullptr),     method("b", "a", "func()", nullptr)};
    CHECK(methods_sorted(ok, 6));
    CHECK(methods_sorted(ok, 1) && methods_sorted(ok, 0));
    const method swapped[] = {method("B", nullptr, "func()", nullptr),
                              method("A", nullptr, "func()", nullptr)};
    CHECK(!methods_sorted(swapped, 2));
    // An unexported name before an exported one is wrong, however the names compare.
    const method unexported_first[] = {method("Z", "p", "func()", nullptr),
                                       method("a", nullptr, "func()", nullptr)};
    CHECK(!methods_sorted(unexported_first, 2));
    const method pkgs[] = {method("a", "q", "func()", nullptr),
                           method("a", "p", "func()", nullptr)};
    CHECK(!methods_sorted(pkgs, 2));
    const method dup[] = {method("A", nullptr, "func()", nullptr),
                          method("A", nullptr, "func()", nullptr)};
    CHECK(!methods_sorted(dup, 2));
    const imethod iok[] = {imethod("A", nullptr, "func()"), imethod("m", "a", "func()"),
                           imethod("m", "b", "func()")};
    CHECK(imethods_sorted(iok, 3));
    const imethod ibad[] = {imethod("m", "a", "func()"), imethod("A", nullptr, "func()")};
    CHECK(!imethods_sorted(ibad, 2));
}

void test_foreign() {
    using namespace gp;
    const type_desc* ti = foreign_td<int>();
    CHECK_NAME(ti, "int");
    CHECK(ti->gp_kind == kind::Foreign && !ti->gp_named && ti->gp_pkg == nullptr);
    CHECK(ti->gp_size == sizeof(int) && ti->gp_align == alignof(int));
    CHECK(ti == foreign_td<int>());
    // Qualifiers and references don't make another type.
    CHECK(ti == foreign_td<const int>() && ti == foreign_td<int&>() &&
          ti == foreign_td<const int&>());
    // Another C++ type is another descriptor, even where it has the same size (a C long and
    // a C long long are 8 bytes each), and a Go int is not a C int.
    CHECK(foreign_td<long>() != ti && foreign_td<long>() != foreign_td<long long>());
    CHECK(ti != &td_int32 && ti != &td_int);
    CHECK_NAME(foreign_td<long>(), "long");
    CHECK_NAME(foreign_td<unsigned char>(), "unsigned char");
    CHECK_NAME(foreign_td<double>(), "double");
    // (const and volatile of the pointees are dropped: Go has no const, step 5.6.)
    CHECK_NAME(foreign_td<const char*>(), "char*");
    CHECK_NAME(foreign_td<const char* const*>(), "char**");
    CHECK_NAME(foreign_td<void*>(), "void*");
    CHECK_NAME(foreign_td<std::nullptr_t>(), "decltype(nullptr)");
    CHECK(std::string(foreign_td<Point>()->gp_name).find("Point") != std::string::npos);
    CHECK(std::string(foreign_td<std::string>()->gp_name).find("string") != std::string::npos);
    CHECK(std::string(foreign_td<std::vector<long>>()->gp_name).find("vector<long") !=
          std::string::npos);

    // == and the hash: what the C++ type supports.
    CHECK(td_equal(ti, 3, 3) && !td_equal(ti, 3, 4));
    CHECK(td_hash(ti, 3) == td_hash(ti, 3));
    CHECK(td_equal(foreign_td<double>(), 1.5, 1.5) &&
          !td_equal(foreign_td<double>(), std::nan(""), std::nan("")));
    int a = 0, b = 0;
    const int* pa = &a;
    const int* pb = &b;
    // A raw pointer compares and hashes by address.
    const type_desc* tp = foreign_td<const int*>();
    CHECK(td_equal(tp, pa, pa) && !td_equal(tp, pa, pb));
    CHECK(td_hash(tp, pa) == td_hash(tp, pa));
    const char* s1 = "abc";
    const char* s2 = "abc";
    const char* s3 = s1;
    CHECK(td_equal(foreign_td<const char*>(), s1, s3));
    (void)s2; // (two equal literals may or may not share an address: not tested)
    CHECK(td_equal(foreign_td<std::nullptr_t>(), nullptr, nullptr));
    CHECK(td_equal(foreign_td<std::string>(), std::string("a"), std::string("a")) &&
          !td_equal(foreign_td<std::string>(), std::string("a"), std::string("b")));
    CHECK(td_hash(foreign_td<std::string>(), std::string("a")) ==
          td_hash(foreign_td<std::string>(), std::string("a")));
    CHECK(td_equal(foreign_td<Point>(), Point{1, 2}, Point{1, 2}) &&
          !td_equal(foreign_td<Point>(), Point{1, 2}, Point{2, 1}));
    CHECK(td_hash(foreign_td<Point>(), Point{1, 2}) == td_hash(foreign_td<Point>(), Point{1, 2}));
    CHECK(td_equal(foreign_td<Color>(), Green, Green) &&
          !td_equal(foreign_td<Color>(), Red, Green));
    CHECK(td_hash(foreign_td<Color>(), Green) == td_hash(foreign_td<Color>(), Green));
    // No == (a C struct): not comparable. == without a std::hash: comparable, and hashable by one
    // hash for all its values (a map key of Go's rules, slowly).
    CHECK(foreign_td<Plain>()->gp_equal == nullptr && foreign_td<Plain>()->gp_hash == nullptr);
    CHECK(foreign_td<Hashed>()->gp_equal != nullptr && foreign_td<Hashed>()->gp_hash != nullptr);
    CHECK(td_hash(foreign_td<Hashed>(), Hashed{1}) == td_hash(foreign_td<Hashed>(), Hashed{2}));
    CHECK(foreign_td<std::vector<long>>()->gp_equal != nullptr &&
          foreign_td<std::vector<long>>()->gp_hash != nullptr);

    // Foreign descriptors are the parts of composite ones: a slice of C ints is not a slice of
    // Go int32s.
    const type_desc* s_c = td_slice<int>(ti);
    CHECK_NAME(s_c, "[]int");
    CHECK(s_c != td_slice<int32_t>(&td_int32) && s_c != td_slice<int64_t>(&td_int));
    CHECK(s_c == td_slice<int>(foreign_td<int>()));

    // From many threads at once, one descriptor.
    const type_desc* seen[8] = {};
    std::vector<std::thread> threads;
    for (int i = 0; i < 8; i++) {
        threads.emplace_back([&seen, i] { seen[i] = foreign_td<std::set<int>>(); });
    }
    for (auto& t : threads) {
        t.join();
    }
    for (int i = 1; i < 8; i++) {
        CHECK(seen[i] == seen[0]);
    }
}

void test_opaque() {
    using namespace gp;
    const type_desc* ci = td_opaque("C.int");
    CHECK_NAME(ci, "C.int");
    CHECK(ci->gp_kind == kind::Opaque);
    CHECK(ci == td_opaque("C.int"));
    // The spelling is copied: a temporary string is the same type as a literal.
    CHECK(ci == td_opaque(std::string("C.i").append("nt").c_str()));
    CHECK(ci != td_opaque("C.double"));
    CHECK(ci != td_opaque("C.int "));
    CHECK(td_opaque("C.double") == td_opaque("C.double"));
    // In a signature: two types that go/types can't tell apart are two descriptors.
    using F = gp::func<void()>;
    const type_desc* f_int = td_func<F>({}, {ci});
    const type_desc* f_double = td_func<F>({}, {td_opaque("C.double")});
    CHECK(f_int != f_double);
    CHECK_NAME(f_int, "func() C.int");
    CHECK_NAME(f_double, "func() C.double");
    CHECK(f_int == td_func<F>({}, {td_opaque("C.int")}));

    // From many threads at once: 300 spellings in a different order in each thread, which makes
    // the table grow several times while the others read it.
    constexpr int threads_n = 8;
    constexpr int names_n = 300;
    const std::size_t before = detail::interned_count();
    std::vector<std::vector<const type_desc*>> got(threads_n,
                                                   std::vector<const type_desc*>(names_n));
    std::vector<std::thread> threads;
    for (int t = 0; t < threads_n; t++) {
        threads.emplace_back([&got, t] {
            for (int k = 0; k < names_n; k++) {
                int i = (k * 7 + t * 31) % names_n;
                got[t][i] = td_opaque(("T" + std::to_string(i)).c_str());
                // Also a composite built from it, and a lookup of what other threads made.
                (void)td_slice<int>(got[t][i]);
            }
        });
    }
    for (auto& t : threads) {
        t.join();
    }
    std::set<const type_desc*> distinct;
    for (int i = 0; i < names_n; i++) {
        for (int t = 1; t < threads_n; t++) {
            CHECK(got[t][i] == got[0][i]);
        }
        CHECK_NAME(got[0][i], "T" + std::to_string(i));
        CHECK(td_slice<int>(got[0][i])->gp_elem == got[0][i]);
        distinct.insert(got[0][i]);
    }
    CHECK(distinct.size() == names_n);
    CHECK(detail::interned_count() == before + 2 * names_n); // each spelling and its slice, once
}

// Every kind has a descriptor in this file.
void test_every_kind() {
    using namespace gp;
    std::set<kind> seen;
    const type_desc* all[] = {&td_bool,
                              &td_int,
                              &td_int8,
                              &td_int16,
                              &td_int32,
                              &td_int64,
                              &td_uint,
                              &td_uint8,
                              &td_uint16,
                              &td_uint32,
                              &td_uint64,
                              &td_uintptr,
                              &td_float32,
                              &td_float64,
                              &td_complex64,
                              &td_complex128,
                              &td_string,
                              &td_unsafe_pointer,
                              &td_any,
                              td_array<int64_t, 2>(&td_int),
                              td_chan<fake_chan<int64_t>>(chan_dir::both, &td_int),
                              td_func<gp::func<void()>>({}, {}),
                              td_map<int64_t, int64_t>(&td_int, &td_int),
                              td_ptr<int64_t>(&td_int),
                              td_slice<int64_t>(&td_int),
                              td_struct<Empty>({}),
                              foreign_td<int>(),
                              td_opaque("C.long")};
    for (const type_desc* td : all) {
        seen.insert(td->gp_kind);
    }
    for (auto k = static_cast<int>(kind::Bool); k <= static_cast<int>(kind::Opaque); k++) {
        CHECK(seen.count(static_cast<kind>(k)) == 1);
    }
    CHECK(seen.count(kind::Invalid) == 0);
}

void run_tests() {
    test_predeclared();
    test_names();
    test_interning();
    test_named();
    test_method_order();
    test_foreign();
    test_opaque();
    test_every_kind();
    rt_test::threads_ended(); // see threads_ended.hpp
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
