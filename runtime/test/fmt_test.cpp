// The unit tests of the fmt shim (gocpp/fmtshim.hpp) for what the differential test
// (test/fmt_diff.cpp against runtime/testdata/fmtdiff/main.go) can't compare with real Go:
// addresses (also of weak pointers), the order of pointer keys, which types count as having
// String and Error methods (checked at compile time), the type names of the tags,
// argument-dependent lookup, and the speed of a big slice. runtime/cxx_test.go compiles this
// file with the runtime's .cpp files, with g++ and clang++, at -O0, at -O2 and with ASan + UBSan,
// and runs it like test/unit.cpp.
// It prints "ok: N checks" and exits 0, or prints each failed check and exits 1.
#include <gocpp/rt.hpp>
#include <gocpp/os.hpp>

#include <algorithm>
#include <chrono>
#include <cstdint>
#include <cstdio>
#include <string>
#include <vector>

namespace fmt = gp::fmtshim;
namespace tag = gp::fmtshim::tag;
using gp::fmtshim::typed;

// A user's package with Go functions whose names are the names of the shim's helpers. The
// shim's calls are qualified, so argument-dependent lookup never picks them (test_adl).
namespace gpkg_adl {

struct P {
    int64_t v = 0;
    bool operator==(const P&) const = default;
    std::size_t gp_hash() const { return gp::hash_fields(v); }
    static constexpr const char* gp_type_name = "adl.P";
    static constexpr auto gp_fields = gp::fields(gp::field(&P::v, "v", false, false));
};

int64_t hijacked = 0;

template <typename... A>
void hijack(const A&...) {
    hijacked++;
}

void compare(const P&, const P&) {
    hijack();
}
void print_value(const P&) {
    hijack();
}
void print_kind(const P&) {
    hijack();
}
void print_slice(const gp::slice<P>&) {
    hijack();
}
void print_map(const gp::map<P, P>&) {
    hijack();
}
void print_struct(const P&) {
    hijack();
}
void print_pointer(const gp::ptr<P>&) {
    hijack();
}
void handle_methods(const P&) {
    hijack();
}
void type_name(const P&) {
    hijack();
}
void base_value(const P&) {
    hijack();
}
void make_arg(const P&) {
    hijack();
}

} // namespace gpkg_adl

namespace gpkg_main {

struct Point {
    int64_t X = 0;
    int64_t Y = 0;
    static constexpr const char* gp_type_name = "main.Point";
    static constexpr auto gp_fields =
        gp::fields(gp::field(&Point::X, "X", true, false), gp::field(&Point::Y, "Y", true, false));
};

// A recursive type: gp_fields is written while Node is incomplete.
struct Node {
    int64_t V = 0;
    gp::slice<Node> Kids;
    gp::ptr<Node> Next;
    gp::map<gp::string, Node> ByName;
    static constexpr const char* gp_type_name = "main.Node";
    static constexpr auto gp_fields = gp::fields(gp::field(&Node::V, "V", true, false),
                                                 gp::field(&Node::Kids, "Kids", true, false),
                                                 gp::field(&Node::Next, "Next", true, false),
                                                 gp::field(&Node::ByName, "ByName", true, false));
};

// Go methods named String that are not fmt's String() string: a field of func type named
// String, a String with a parameter, a String that returns an int. Without gp_String, they
// don't count (the emitter writes gp_String only from Go's method sets).
struct FieldString {
    int64_t (*String)() = nullptr;
    static constexpr const char* gp_type_name = "main.FieldString";
    static constexpr auto gp_fields = gp::fields();
};

struct WrongString {
    gp::string String(int64_t) const { return "wrong"_gs; }
    int64_t gp_String() const { return 1; } // not a gp::string: not the hook
    static constexpr const char* gp_type_name = "main.WrongString";
    static constexpr auto gp_fields = gp::fields();
};

struct ValueString {
    gp::string gp_String() const { return "value"_gs; }
    static constexpr const char* gp_type_name = "main.ValueString";
    static constexpr auto gp_fields = gp::fields();
};

struct PtrString {
    static gp::string gp_String_ptr(const gp::ptr<PtrString>& p) {
        return p == nullptr ? "nil"_gs : "ptr"_gs;
    }
    static constexpr const char* gp_type_name = "main.PtrString";
    static constexpr auto gp_fields = gp::fields();
};

struct PtrError {
    static gp::string gp_Error_ptr(const gp::ptr<PtrError>&) { return "err"_gs; }
    gp::string gp_String() const { return "str"_gs; }
    static constexpr const char* gp_type_name = "main.PtrError";
    static constexpr auto gp_fields = gp::fields();
};

namespace hooks = gp::fmtshim::detail;
static_assert(!hooks::methods<FieldString>::string && !hooks::methods<WrongString>::string);
static_assert(hooks::methods<ValueString>::string && hooks::methods<gp::ptr<ValueString>>::string);
static_assert(!hooks::methods<PtrString>::string && hooks::methods<gp::ptr<PtrString>>::string);
static_assert(!hooks::methods<PtrError>::error && hooks::methods<gp::ptr<PtrError>>::error);
static_assert(!hooks::methods<gp::ptr<gp::ptr<ValueString>>>::string,
              "a pointer to a pointer has no methods");
static_assert(!hooks::methods<gp::slice<ValueString>>::string);

} // namespace gpkg_main

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

std::string str(const gp::string& s) {
    return std::string(s.gp_view());
}

std::string hex(const void* p) {
    char buf[32];
    std::snprintf(buf, sizeof buf, "0x%llx",
                  static_cast<unsigned long long>(reinterpret_cast<std::uintptr_t>(p)));
    return buf;
}

std::string dec(const void* p) {
    return std::to_string(static_cast<unsigned long long>(reinterpret_cast<std::uintptr_t>(p)));
}

using gpkg_main::Node;
using gpkg_main::Point;

// Pointers print their addresses: at the top level only a pointer to a composite prints &{...}.
void test_addresses() {
    gp::ptr<int64_t> x = gp::new_<int64_t>(5);
    std::string a = hex(x.gp_get());
    CHECK_EQ(str(fmt::Sprint(x)), a);
    CHECK_EQ(str(fmt::Sprintf("%v|%d|%s"_gs, x, x, x)),
             a + "|" + dec(x.gp_get()) + "|%!s(*int=" + a + ")");
    CHECK_EQ(str(fmt::Sprint(gp::slice_of<gp::ptr<int64_t>>({x, nullptr}))), "[" + a + " <nil>]");
    CHECK_EQ(str(fmt::Sprintf("%s"_gs, gp::slice_of<gp::ptr<int64_t>>({x}))),
             "[%!s(*int=" + a + ")]");
    CHECK_EQ(str(fmt::Sprintf("%+v"_gs, typed<tag::ptr<tag::int64>>(x))), a);
    CHECK_EQ(str(fmt::Sprintf("%t"_gs, typed<tag::ptr<tag::int64>>(x))), "%!t(*int64=" + a + ")");
    // Widths pad the address; the zero flag pads its digits, after the 0x.
    CHECK_EQ(str(fmt::Sprintf("%30v|"_gs, x)), std::string(30 - a.size(), ' ') + a + "|");
    CHECK_EQ(str(fmt::Sprintf("%-30v|"_gs, x)), a + std::string(30 - a.size(), ' ') + "|");
    CHECK_EQ(str(fmt::Sprintf("%030v"_gs, x)),
             "0x" + std::string(30 - (a.size() - 2), '0') + a.substr(2));

    // Nested pointers: inside a struct, a slice, a map, a pointer.
    gp::ptr<Point> p = gp::new_<Point>(Point{1, 2});
    gp::ptr<gp::ptr<Point>> pp = gp::new_<gp::ptr<Point>>(p);
    CHECK_EQ(str(fmt::Sprint(p)), "&{1 2}");
    CHECK_EQ(str(fmt::Sprint(pp)), hex(pp.gp_get()));
    CHECK_EQ(str(fmt::Sprint(gp::slice_of<gp::ptr<Point>>({p}))), "[" + hex(p.gp_get()) + "]");
    CHECK_EQ(str(fmt::Sprint(gp::map_of<gp::string, gp::ptr<Point>>({{"k"_gs, p}}))),
             "map[k:" + hex(p.gp_get()) + "]");
    Node n{1, gp::slice_of<Node>({Node{2, nullptr, nullptr, nullptr}}),
           gp::new_<Node>(Node{3, nullptr, nullptr, nullptr}), nullptr};
    CHECK_EQ(str(fmt::Sprint(n)), "{1 [{2 [] <nil> map[]}] " + hex(n.Next.gp_get()) + " map[]}");
    CHECK_EQ(str(fmt::Sprintf("%+v"_gs, gp::new_<Node>(n))),
             "&{V:1 Kids:[{V:2 Kids:[] Next:<nil> ByName:map[]}] Next:" + hex(n.Next.gp_get()) +
                 " ByName:map[]}");
    // A pointer to a pointer is not dereferenced: its address.
    gp::ptr<gp::ptr<gp::slice<int64_t>>> ps =
        gp::new_<gp::ptr<gp::slice<int64_t>>>(gp::new_<gp::slice<int64_t>>());
    CHECK_EQ(str(fmt::Sprint(ps)), hex(ps.gp_get()));
}

// Pointer keys sort by address (internal/fmtsort).
void test_pointer_keys() {
    std::vector<gp::ptr<int64_t>> ps;
    gp::map<gp::ptr<int64_t>, int64_t> m = gp::make_map<gp::ptr<int64_t>, int64_t>();
    for (int64_t i = 0; i < 8; i++) {
        ps.push_back(gp::new_<int64_t>(i));
        gp::set(m, ps.back(), i);
    }
    gp::set(m, gp::ptr<int64_t>(), INT64_C(-1));
    std::sort(ps.begin(), ps.end(), [](const gp::ptr<int64_t>& a, const gp::ptr<int64_t>& b) {
        return reinterpret_cast<std::uintptr_t>(a.gp_get()) <
               reinterpret_cast<std::uintptr_t>(b.gp_get());
    });
    std::string want = "map[<nil>:-1";
    for (const auto& p : ps) {
        want += " " + hex(p.gp_get()) + ":" + std::to_string(*p);
    }
    want += "]";
    CHECK_EQ(str(fmt::Sprint(m)), want);
}

// The methods: by value, through a pointer only, Error before String; a wrong String never.
void test_methods() {
    using namespace gpkg_main;
    CHECK_EQ(str(fmt::Sprint(FieldString{}, WrongString{})), "{} {}");
    CHECK_EQ(str(fmt::Sprint(ValueString{}, gp::new_<ValueString>(), gp::ptr<ValueString>())),
             "value value <nil>");
    CHECK_EQ(str(fmt::Sprint(PtrString{}, gp::new_<PtrString>(), gp::ptr<PtrString>())),
             "{} ptr nil");
    CHECK_EQ(str(fmt::Sprint(PtrError{}, gp::new_<PtrError>())), "str err");
    CHECK_EQ(str(fmt::Sprintf("%d|%5s|%-5v|%q"_gs, ValueString{}, ValueString{}, ValueString{},
                              ValueString{})),
             "{}|value|value|\"value\"");
    // Elements: the methods of each element's type.
    CHECK_EQ(str(fmt::Sprint(gp::slice_of<gp::ptr<PtrString>>({nullptr}),
                             gp::slice_of<PtrString>({PtrString{}}))),
             "[nil] [{}]");
}

// The Go type names of the error forms and %T, with the tags.
void test_type_names() {
    auto T = [](auto v) { return str(fmt::Sprintf("%T"_gs, v)); };
    CHECK_EQ(T(gp::map<gp::string, gp::slice<gp::ptr<Point>>>()), "map[string][]*main.Point");
    CHECK_EQ(T(gp::array<gp::array<uint8_t, 2>, 3>{}), "[3][2]uint8");
    CHECK_EQ(T(gp::slice<gp::map<int32_t, float>>()), "[]map[int32]float32");
    CHECK_EQ(T(typed<tag::map<tag::uintptr, tag::slice<tag::ptr<tag::uint64>>>>(
                 gp::map<uint64_t, gp::slice<gp::ptr<uint64_t>>>())),
             "map[uintptr][]*uint64");
    CHECK_EQ(T(typed<tag::map<tag::none, tag::int64>>(gp::map<int64_t, int64_t>())),
             "map[int]int64");
    CHECK_EQ(T(Node{}), "main.Node");
    CHECK_EQ(T(gp::new_<Node>()), "*main.Node");
    CHECK_EQ(T(int16_t(1)), "int16");
    CHECK_EQ(T(uint32_t(1)), "uint32");
    CHECK_EQ(T(uint64_t(1)), "uint");
}

// Weak pointers print as Go's weak.Pointer struct, { _ [0]*T; u unsafe.Pointer }, with the
// address they were made from (also after the target is gone); they sort by it as map keys.
void test_weak() {
    gp::ptr<Point> p = gp::new_<Point>(Point{1, 2});
    gp::weak_ptr<Point> w = gp::weak_make(p);
    gp::weak_ptr<Point> z;
    std::string a = hex(p.gp_get());
    CHECK_EQ(str(fmt::Sprint(w, z)), "{[] " + a + "} {[] <nil>}");
    CHECK_EQ(str(fmt::Sprintf("%+v|%d|%s|%d"_gs, w, w, w, z)),
             "{_:[] u:" + a + "}|{[] " + dec(p.gp_get()) + "}|{[] %!s(unsafe.Pointer=" + a +
                 ")}|{[] 0}");
    CHECK_EQ(str(fmt::Sprintf("%7v|%-7v|%q"_gs, z, z, z)),
             "{[]   <nil>}|{[] <nil>  }|{[] %!q(unsafe.Pointer=<nil>)}");
    CHECK_EQ(str(fmt::Sprint(gp::slice_of<gp::weak_ptr<Point>>({w, z}))),
             "[{[] " + a + "} {[] <nil>}]");
    // A pointer to a weak pointer prints &{...} at the top level, as one to a struct, and its
    // address below it.
    gp::ptr<gp::weak_ptr<Point>> pw = gp::new_<gp::weak_ptr<Point>>(w);
    CHECK_EQ(str(fmt::Sprintf("%v|%+v"_gs, pw, gp::new_<gp::weak_ptr<Point>>())),
             "&{[] " + a + "}|&{_:[] u:<nil>}");
    CHECK_EQ(str(fmt::Sprint(gp::slice_of<gp::ptr<gp::weak_ptr<Point>>>({pw}))),
             "[" + hex(pw.gp_get()) + "]");
    p = nullptr; // the target is gone: the weak pointer keeps its address
    CHECK_EQ(str(fmt::Sprint(w)), "{[] " + a + "}");
    CHECK(gp::weak_value(w) == nullptr);

    std::vector<gp::ptr<int64_t>> ps;
    gp::map<gp::weak_ptr<int64_t>, int64_t> m = gp::make_map<gp::weak_ptr<int64_t>, int64_t>();
    for (int64_t i = 0; i < 6; i++) {
        ps.push_back(gp::new_<int64_t>(i));
        gp::set(m, gp::weak_make(ps.back()), i);
    }
    gp::set(m, gp::weak_ptr<int64_t>(), INT64_C(-1));
    std::sort(ps.begin(), ps.end(), [](const gp::ptr<int64_t>& x, const gp::ptr<int64_t>& y) {
        return reinterpret_cast<std::uintptr_t>(x.gp_get()) <
               reinterpret_cast<std::uintptr_t>(y.gp_get());
    });
    std::string want = "map[{[] <nil>}:-1";
    for (const auto& q : ps) {
        want += " {[] " + hex(q.gp_get()) + "}:" + std::to_string(*q);
    }
    CHECK_EQ(str(fmt::Sprint(m)), want + "]");

    auto T = [](auto v) { return str(fmt::Sprintf("%T"_gs, v)); };
    CHECK_EQ(T(z), "weak.Pointer[main.Point]");
    CHECK_EQ(T(gp::slice<gp::weak_ptr<gp::ptr<Point>>>()), "[]weak.Pointer[*main.Point]");
    CHECK_EQ(T(gp::weak_ptr<int64_t>()), "weak.Pointer[int]");
    CHECK_EQ(T(typed<tag::weak<tag::int64>>(gp::weak_ptr<int64_t>())), "weak.Pointer[int64]");
    CHECK_EQ(T(typed<tag::map<tag::none, tag::weak<tag::uint64>>>(
                 gp::map<gp::string, gp::weak_ptr<uint64_t>>())),
             "map[string]weak.Pointer[uint64]");
}

// typed() copies the value, so a typed argument can be kept in a temporary.
void test_typed_copy() {
    gp::slice<int64_t> s = gp::slice_of<int64_t>({1, 2});
    auto t = typed<tag::slice<tag::int64>>(s);
    s = nullptr;
    CHECK_EQ(str(fmt::Sprintf("%v %s"_gs, t, t)), "[1 2] [%!s(int64=1) %!s(int64=2)]");
}

// The shim's calls are qualified: a user's functions of the same names are never called.
void test_adl() {
    using gpkg_adl::P;
    auto m = gp::map_of<P, P>({{P{2}, P{1}}, {P{1}, P{2}}});
    std::string got =
        str(fmt::Sprint(P{1}, gp::slice_of<P>({P{2}}), m, gp::new_<P>(P{3}), gp::ptr<P>()));
    CHECK_EQ(got, "{1} [{2}] map[{1}:{2} {2}:{1}] &{3} <nil>");
    CHECK_EQ(str(fmt::Sprintf("%d %T %x"_gs, P{4}, P{5}, P{6})), "{4} adl.P {%!x(int=6)}");
    CHECK_EQ(gpkg_adl::hijacked, INT64_C(0));
}

// Sprint gives a string that owns its bytes; a big width or precision works.
void test_sprint() {
    gp::string s = fmt::Sprint(INT64_C(12), "ab"_gs);
    CHECK_EQ(str(s), "12ab");
    CHECK(s.gp_owner() != nullptr);
    CHECK_EQ(str(fmt::Sprint()), "");
    CHECK_EQ(gp::len(fmt::Sprintf("%1000000d"_gs, INT64_C(7))), INT64_C(1000000));
    CHECK_EQ(gp::len(fmt::Sprintf("%.1000000d"_gs, INT64_C(7))), INT64_C(1000000));
    CHECK_EQ(gp::len(fmt::Sprintf("%.1000000f"_gs, 0.5)), INT64_C(1000002));
    CHECK_EQ(gp::len(fmt::Sprintf("%.1000000v"_gs, 0.1)) > 50, true);
    CHECK_EQ(str(fmt::Sprintf("%.60v"_gs, 0.1)),
             "0.1000000000000000055511151231257827021181583404541015625");
}

// Println of a 1e5-element []int, the speed check of the plan (Go: about 2-4 ms): the output
// grows linearly. Reported, not checked (the machines differ), except a generous bound.
double sprint_ms(int64_t n) {
    gp::slice<int64_t> s = gp::make_slice<int64_t>(n);
    for (int64_t i = 0; i < n; i++) {
        s[i] = i * 7919 % 100003;
    }
    auto t0 = std::chrono::steady_clock::now();
    gp::string out = fmt::Sprintln(s);
    auto t1 = std::chrono::steady_clock::now();
    CHECK(gp::len(out) > n);
    return std::chrono::duration<double, std::milli>(t1 - t0).count();
}

std::string test_speed() {
    double small = sprint_ms(100000);
    double big = sprint_ms(1000000);
    CHECK(big < 10000); // even with ASan, 1e6 elements take well under a second
    char buf[128];
    std::snprintf(buf, sizeof buf, "Sprintln of 1e5 ints: %.1f ms, 1e6: %.1f ms", small, big);
    return buf;
}

void run_tests() {
    test_addresses();
    test_pointer_keys();
    test_methods();
    test_type_names();
    test_typed_copy();
    test_weak();
    test_adl();
    test_sprint();
    std::string speed = test_speed();
    if (failures > 0) {
        std::string msg =
            "FAIL: " + std::to_string(failures) + " of " + std::to_string(checks) + " checks\n";
        gp::os::write(gp::os::stderr_fd, msg.data(), msg.size());
        gp::os::exit(1);
    }
    fmt::Printf("ok: %d checks (%s)\n"_gs, int64_t(checks), gp::string::gp_copy(speed));
}

void no_init() {}

} // namespace

int main(int argc, char** argv) {
    return gp::run_main(argc, argv, &no_init, &run_tests);
}
