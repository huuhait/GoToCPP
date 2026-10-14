// The C++ side of the differential test of the fmt shim: the program of
// runtime/testdata/fmtdiff/main.go, written against the runtime as gocpp would generate it:
// Go's int is int64_t, a rune int32_t, a byte uint8_t; structs, named types and methods follow
// the protocols of gocpp/fmtshim.hpp, and arguments of type int64, uint64 and uintptr get type
// tags. runtime/cxx_test.go runs both with each case name and compares the exit codes and the
// merged stdout and stderr.
#include <gocpp/rt.hpp>
#include <gocpp/os.hpp>

#include <cmath>
#include <cstdint>
#include <limits>
#include <string_view>

namespace gpkg_main {

namespace fmt = gp::fmtshim;
namespace tag = gp::fmtshim::tag;
using gp::fmtshim::typed;

const double inf = std::numeric_limits<double>::infinity();
const double nan = std::numeric_limits<double>::quiet_NaN();
const double pi = 3.141592653589793;
const double e = 2.718281828459045;

template <typename T>
gp::slice<T> S(std::initializer_list<T> v) {
    return gp::slice_of<T>(v);
}

template <typename K, typename V>
gp::map<K, V> M(std::initializer_list<std::pair<K, V>> v) {
    return gp::map_of<K, V>(v);
}

gp::slice<uint8_t> bytes(const gp::string& s) {
    return gp::append_slice(gp::slice_of<uint8_t>({}), s);
}

gp::string itoa(int64_t v) {
    return fmt::Sprint(v);
}

// --- the types ---

struct Point {
    int64_t X = 0;
    int64_t Y = 0;
    bool operator==(const Point&) const = default;
    std::size_t gp_hash() const { return gp::hash_fields(X, Y); }
    static constexpr const char* gp_type_name = "main.Point";
    static constexpr auto gp_fields =
        gp::fields(gp::field(&Point::X, "X", true, false), gp::field(&Point::Y, "Y", true, false));
};

struct inner {
    int64_t A = 0;
    int64_t b = 0;
    static constexpr const char* gp_type_name = "main.inner";
    static constexpr auto gp_fields =
        gp::fields(gp::field(&inner::A, "A", true, false), gp::field(&inner::b, "b", false, false));
};

struct Inner {
    int64_t N = 0;
    static constexpr const char* gp_type_name = "main.Inner";
    static constexpr auto gp_fields = gp::fields(gp::field(&Inner::N, "N", true, false));
};

struct Mixed {
    gp::string Name;
    gp::slice<gp::string> Tags;
    gp::map<gp::string, int64_t> M;
    gp::ptr<Point> P;
    inner in;
    Inner Inner_; // embedded
    int64_t gp_blank0 = 0;
    double F = 0;
    Point Pt;
    static constexpr const char* gp_type_name = "main.Mixed";
    static constexpr auto gp_fields = gp::fields(
        gp::field(&Mixed::Name, "Name", true, false), gp::field(&Mixed::Tags, "Tags", true, false),
        gp::field(&Mixed::M, "M", true, false), gp::field(&Mixed::P, "P", true, false),
        gp::field(&Mixed::in, "in", false, false), gp::field(&Mixed::Inner_, "Inner", true, true),
        gp::field(&Mixed::gp_blank0, "_", false, false), gp::field(&Mixed::F, "F", true, false),
        gp::field(&Mixed::Pt, "Pt", true, false));
};

struct Empty {
    bool operator==(const Empty&) const = default;
    std::size_t gp_hash() const { return gp::hash_fields(); }
    static constexpr const char* gp_type_name = "main.Empty";
    static constexpr auto gp_fields = gp::fields();
};

// A named type over T, as gp::newtype (step 3.9) will be.
template <typename Self, typename T>
struct named {
    T gp_v{};
    named() = default;
    explicit named(T v) : gp_v(std::move(v)) {}
    const T& gp_value() const { return gp_v; }
    friend bool operator==(const Self& a, const Self& b) { return a.gp_v == b.gp_v; }
    std::size_t gp_hash() const { return gp::hash_of(gp_v); }
};

struct Celsius : named<Celsius, double> {
    using named::named;
    static constexpr const char* gp_type_name = "main.Celsius";
    gp::string String() const { return fmt::Sprintf("%.1f°C"_gs, gp_v); }
    gp::string gp_String() const { return String(); }
};

struct Name : named<Name, gp::string> {
    using named::named;
    static constexpr const char* gp_type_name = "main.Name";
};

struct IDs : named<IDs, gp::slice<int64_t>> {
    using named::named;
    static constexpr const char* gp_type_name = "main.IDs";
};

struct Color : named<Color, int64_t> {
    using named::named;
    static constexpr const char* gp_type_name = "main.Color";
    gp::string String() const {
        switch (gp_v) {
        case 0:
            return "red"_gs;
        case 1:
            return "green"_gs;
        }
        return "color("_gs + itoa(gp_v) + ")"_gs;
    }
    gp::string gp_String() const { return String(); }
};

struct PtrS {
    int64_t V = 0;
    static constexpr const char* gp_type_name = "main.PtrS";
    static constexpr auto gp_fields = gp::fields(gp::field(&PtrS::V, "V", true, false));
    static gp::string String(gp::ptr<PtrS> p) {
        if (p == nullptr) {
            return "nil PtrS"_gs;
        }
        return "PtrS("_gs + itoa(p->V) + ")"_gs;
    }
    static gp::string gp_String_ptr(const gp::ptr<PtrS>& p) { return PtrS::String(p); }
};

struct Deref {
    int64_t V = 0;
    static constexpr const char* gp_type_name = "main.Deref";
    static constexpr auto gp_fields = gp::fields(gp::field(&Deref::V, "V", true, false));
    static gp::string String(gp::ptr<Deref> p) { return "Deref("_gs + itoa(p->V) + ")"_gs; }
    static gp::string gp_String_ptr(const gp::ptr<Deref>& p) { return Deref::String(p); }
};

struct Both {
    static constexpr const char* gp_type_name = "main.Both";
    static constexpr auto gp_fields = gp::fields();
    gp::string Error() const { return "the error"_gs; }
    gp::string String() const { return "the string"_gs; }
    gp::string gp_Error() const { return Error(); }
    gp::string gp_String() const { return String(); }
};

struct Wrap {
    Celsius C;
    Celsius c;
    gp::slice<Color> L;
    gp::slice<Color> l;
    static constexpr const char* gp_type_name = "main.Wrap";
    static constexpr auto gp_fields =
        gp::fields(gp::field(&Wrap::C, "C", true, false), gp::field(&Wrap::c, "c", false, false),
                   gp::field(&Wrap::L, "L", true, false), gp::field(&Wrap::l, "l", false, false));
};

struct Outer {
    Color Color_; // embedded
    int64_t N = 0;
    static constexpr const char* gp_type_name = "main.Outer";
    static constexpr auto gp_fields = gp::fields(gp::field(&Outer::Color_, "Color", true, true),
                                                 gp::field(&Outer::N, "N", true, false));
    gp::string gp_String() const { return Color_.String(); } // promoted from Color
};

struct Boom {
    int64_t N = 0;
    static constexpr const char* gp_type_name = "main.Boom";
    static constexpr auto gp_fields = gp::fields(gp::field(&Boom::N, "N", true, false));
    gp::string String() const {
        if (N == 1) {
            gp::panic("boom"_gs);
        }
        if (N == 2) {
            gp::slice<int64_t> s = S<int64_t>({1, 2, 3});
            int64_t i = 5;
            return itoa(s[i]);
        }
        if (N == 3) {
            gp::ptr<Point> p;
            return itoa(p->X);
        }
        return "ok"_gs;
    }
    gp::string gp_String() const { return String(); }
};

struct Key {
    gp::string A;
    int64_t B = 0;
    bool operator==(const Key&) const = default;
    std::size_t gp_hash() const { return gp::hash_fields(A, B); }
    static constexpr const char* gp_type_name = "main.Key";
    static constexpr auto gp_fields =
        gp::fields(gp::field(&Key::A, "A", true, false), gp::field(&Key::B, "B", true, false));
};

struct Sizes {
    int64_t I64 = 0;
    uint64_t U64 = 0;
    uint64_t UP = 0;
    gp::slice<int64_t> L;
    static constexpr const char* gp_type_name = "main.Sizes";
    static constexpr auto gp_fields =
        gp::fields(gp::field<tag::int64>(&Sizes::I64, "I64", true, false),
                   gp::field<tag::uint64>(&Sizes::U64, "U64", true, false),
                   gp::field<tag::uintptr>(&Sizes::UP, "UP", true, false),
                   gp::field<tag::slice<tag::int64>>(&Sizes::L, "L", true, false));
};

gp::slice<int64_t> later = S<int64_t>({1, 2});

struct Stamp : named<Stamp, int64_t> {
    using named::named;
    static constexpr const char* gp_type_name = "main.Stamp";
    gp::string String() const {
        later = nullptr;
        return "stamp"_gs;
    }
    gp::string gp_String() const { return String(); }
};

struct MapEdit;
extern gp::map<int64_t, MapEdit> edited;

struct MapEdit : named<MapEdit, int64_t> {
    using named::named;
    static constexpr const char* gp_type_name = "main.MapEdit";
    gp::string String() const {
        for (auto it = gp::range_map(edited); gp::range_next(it);) {
            int64_t k = it.gp_key();
            gp::erase(edited, k);
        }
        return "edit"_gs + itoa(gp_v);
    }
    gp::string gp_String() const { return String(); }
};

gp::map<int64_t, MapEdit> edited;

// Local types, hoisted (Phase 03 §2.4).
struct B : named<B, uint8_t> {
    using named::named;
    static constexpr const char* gp_type_name = "main.B";
};

struct Named : named<Named, gp::ptr<Point>> {
    using named::named;
    static constexpr const char* gp_type_name = "main.Named";
};

// Anonymous struct types (the emitter gives them one generated name each, Phase 03 §2.4).
struct gp_anon_empty {
    static constexpr const char* gp_type_name = "struct {}";
    static constexpr auto gp_fields = gp::fields();
};

// Clear's String empties held.L, Drop's drops.
struct Clear : named<Clear, int64_t> {
    using named::named;
    static constexpr const char* gp_type_name = "main.Clear";
    gp::string String() const;
    gp::string gp_String() const { return String(); }
};

struct Holder {
    gp::slice<Clear> L;
    static constexpr const char* gp_type_name = "main.Holder";
    static constexpr auto gp_fields = gp::fields(gp::field(&Holder::L, "L", true, false));
};

Holder held;

gp::string Clear::String() const {
    held.L = nullptr;
    return "c"_gs;
}

struct Drop : named<Drop, int64_t> {
    using named::named;
    static constexpr const char* gp_type_name = "main.Drop";
    gp::string String() const;
    gp::string gp_String() const { return String(); }
};

gp::slice<gp::slice<Drop>> drops;

gp::string Drop::String() const {
    drops = nullptr;
    return "d"_gs;
}

// --- the cases ---

void slices_case() {
    fmt::Println(S<int64_t>({1, 2, 3}), gp::slice_of<int64_t>({}), gp::slice<int64_t>(),
                 gp::array<int64_t, 3>{4, 5, 6}, gp::array<int64_t, 0>{});
    fmt::Println(S<gp::string>({"a"_gs, "b c"_gs, ""_gs}), S<bool>({true, false}),
                 S<gp::slice<int64_t>>({S<int64_t>({1, 2}), gp::slice_of<int64_t>({}),
                                        gp::slice<int64_t>(), S<int64_t>({3})}));
    fmt::Println(S<int8_t>({-1, 2}), S<uint16_t>({65535}), S<int32_t>({'a', 'b'}),
                 S<uint8_t>({1, 2, 255}));
    fmt::Println(
        S<gp::slice<gp::string>>({S<gp::string>({"x"_gs}), S<gp::string>({"y"_gs, "z"_gs})}),
        gp::array<gp::array<int64_t, 2>, 2>{gp::array<int64_t, 2>{1, 2},
                                            gp::array<int64_t, 2>{3, 4}});
    gp::slice<uint8_t> b = bytes("hi\x00\xc3\xa9"_gs);
    fmt::Printf("%v %d %s %q|%8s|%-8s|%.2s|\n"_gs, b, b, b, b, b, b, b);
    gp::slice<uint8_t> nb;
    fmt::Printf("%v %s %q|%3s|\n"_gs, nb, nb, nb, nb);
    gp::array<uint8_t, 3> arr{'a', 'b', 'c'};
    fmt::Printf("%v %s %q %d\n"_gs, arr, arr, arr, arr);
    gp::slice<B> bs = S<B>({B('x'), B('y')});
    fmt::Printf("%v %s %q\n"_gs, bs, bs, bs);
    fmt::Printf("%5d|%-4d|%03d|%+d\n"_gs, S<int64_t>({1, -2}), S<int64_t>({3}), S<int64_t>({4, 5}),
                S<int64_t>({6}));
    fmt::Printf("%t %s %d\n"_gs, S<int64_t>({1}), S<int64_t>({2, 3}), S<gp::string>({"a"_gs}));
    fmt::Printf("%q %v %s\n"_gs, S<gp::string>({"a\n"_gs, "b"_gs}), S<gp::string>({"x"_gs}),
                S<gp::string>({"y"_gs}));
    fmt::Printf("%q %v %d\n"_gs, S<int32_t>({'x', 0x263A}), S<int32_t>({'x'}), S<int32_t>({'y'}));
    fmt::Printf("%6.2f|%v|%.1v\n"_gs, S<double>({1.5, -2.25}), S<double>({0.1, 1e21}),
                S<double>({1.25}));
    using L1 = gp::slice<int64_t>;
    using L2 = gp::slice<L1>;
    using L3 = gp::slice<L2>;
    using L4 = gp::slice<L3>;
    using L5 = gp::slice<L4>;
    using L6 = gp::slice<L5>;
    using L7 = gp::slice<L6>;
    using L8 = gp::slice<L7>;
    using L9 = gp::slice<L8>;
    using L10 = gp::slice<L9>;
    L10 deep = S<L9>(
        {S<L8>({S<L7>({S<L6>({S<L5>({S<L4>({S<L3>({S<L2>({S<L1>({S<int64_t>({1, 2})})})})})})})})}),
         gp::slice_of<L8>({})});
    fmt::Println(deep, gp::len(fmt::Sprint(deep)));
    fmt::Printf("%v|%d|%s\n"_gs, S<L1>({S<int64_t>({1}), S<int64_t>({2, 3})}),
                S<L1>({S<int64_t>({4})}), S<gp::slice<gp::string>>({S<gp::string>({"s"_gs})}));
    fmt::Println(IDs(S<int64_t>({1, 2})), IDs(), S<Name>({Name("n1"_gs), Name("n2"_gs)}));
    fmt::Println(S<Empty>({Empty{}, Empty{}}), S<gp::ptr<int64_t>>({nullptr, nullptr}),
                 gp::array<bool, 2>{});
    fmt::Println(S<gp::map<gp::string, int64_t>>({M<gp::string, int64_t>({{"a"_gs, 1}}), nullptr}),
                 S<gp::array<gp::string, 2>>({gp::array<gp::string, 2>{"p"_gs, "q"_gs}}));
}

void maps_case() {
    fmt::Println(M<gp::string, int64_t>({{"b"_gs, 2}, {"a"_gs, 1}, {"c"_gs, 3}}),
                 M<gp::string, int64_t>({}), gp::map<gp::string, int64_t>());
    fmt::Println(M<int64_t, gp::string>(
        {{3, "c"_gs}, {-1, "z"_gs}, {0, "o"_gs}, {10, "t"_gs}, {-20, "m"_gs}}));
    fmt::Println(M<double, gp::string>({{2.5, "b"_gs},
                                        {-inf, "ninf"_gs},
                                        {inf, "inf"_gs},
                                        {0, "zero"_gs},
                                        {-1.5, "neg"_gs},
                                        {nan, "nan"_gs}}));
    gp::map<double, int64_t> nans = M<double, int64_t>({{1, 1}});
    gp::set(nans, nan, 7);
    gp::set(nans, nan, 7);
    gp::set(nans, nan, 7);
    fmt::Println(nans, gp::len(nans));
    gp::map<double, int64_t> negz = M<double, int64_t>({{std::copysign(0.0, -1.0), 1}});
    fmt::Println(negz);
    fmt::Println(M<bool, int64_t>({{true, 1}, {false, 0}}),
                 M<uint8_t, bool>({{200, true}, {3, false}, {70, true}}));
    using A2 = gp::array<int64_t, 2>;
    fmt::Println(M<A2, gp::string>({{A2{2, 1}, "c"_gs}, {A2{1, 2}, "b"_gs}, {A2{1, 1}, "a"_gs}}));
    fmt::Println(M<Key, int64_t>({{Key{"b"_gs, 1}, 3}, {Key{"a"_gs, 2}, 2}, {Key{"a"_gs, 1}, 1}}));
    fmt::Println(
        M<gp::string, gp::slice<int64_t>>({{"x"_gs, S<int64_t>({1, 2})}, {"w"_gs, nullptr}}),
        M<gp::string, gp::map<gp::string, int64_t>>(
            {{"o"_gs, M<gp::string, int64_t>({{"z"_gs, 26}, {"a"_gs, 1}})}}));
    fmt::Println(M<int32_t, float>({{'b', 0.1f}, {'a', 2.5f}}),
                 M<int8_t, int64_t>({{-128, 1}, {127, 2}, {0, 3}}));
    fmt::Println(M<uint64_t, int64_t>({{UINT64_MAX, 1}, {0, 2}}),
                 typed<tag::map<tag::int64, tag::none>>(
                     M<int64_t, int64_t>({{INT64_MIN, 1}, {INT64_MAX, 2}})));
    fmt::Printf("%d %s %v %+v\n"_gs, M<gp::string, int64_t>({{"k"_gs, 1}}),
                M<gp::string, int64_t>({{"k"_gs, 2}}),
                M<Name, IDs>({{Name("n"_gs), IDs(S<int64_t>({1}))}}),
                M<gp::string, Point>({{"p"_gs, Point{1, 2}}}));
    fmt::Printf("%5v|%-3d|%q\n"_gs, M<gp::string, int64_t>({{"a"_gs, 1}}),
                M<int64_t, int64_t>({{1, 2}}), M<gp::string, gp::string>({{"k"_gs, "v"_gs}}));
    fmt::Println(
        M<gp::ptr<int64_t>, int64_t>({{nullptr, 5}}),
        M<Celsius, Color>(
            {{Celsius(1.5), Color(1)}, {Celsius(-3), Color(0)}, {Celsius(100), Color(7)}}));
    fmt::Println(M<gp::string, Point>({{"b"_gs, Point{3, 4}}, {"a"_gs, Point{1, 2}}}),
                 M<Point, bool>({{Point{2, 1}, true}, {Point{1, 9}, false}}));
    fmt::Println(M<gp::array<int64_t, 0>, int64_t>({{gp::array<int64_t, 0>{}, 1}}),
                 M<Empty, int64_t>({{Empty{}, 2}}), M<gp::string, Empty>({{"e"_gs, Empty{}}}));
}

struct gp_anon_A {
    gp::slice<Point> A;
    static constexpr const char* gp_type_name = "struct { A []main.Point }";
    static constexpr auto gp_fields = gp::fields(gp::field(&gp_anon_A::A, "A", true, false));
};

struct gp_anon_SN {
    gp::string S;
    gp::ptr<Point> N;
    static constexpr const char* gp_type_name = "struct { S string; N *main.Point }";
    static constexpr auto gp_fields = gp::fields(gp::field(&gp_anon_SN::S, "S", true, false),
                                                 gp::field(&gp_anon_SN::N, "N", true, false));
};

void structs_case() {
    Point p{1, 2};
    fmt::Println(p, Empty{}, inner{3, 4}, gp_anon_empty{});
    fmt::Printf("%v %+v %d %s\n"_gs, p, p, p, p);
    Mixed m;
    m.Name = "m"_gs;
    m.Tags = S<gp::string>({"t1"_gs, "t2"_gs});
    m.M = M<gp::string, int64_t>({{"y"_gs, 2}, {"x"_gs, 1}});
    m.in = inner{5, 6};
    m.Inner_ = Inner{7};
    m.F = 0.5;
    m.Pt = Point{8, 9};
    fmt::Println(m);
    fmt::Printf("%+v\n"_gs, m);
    fmt::Printf("%v|%+v\n"_gs, Mixed{}, Mixed{});
    fmt::Printf("%q\n"_gs, m);
    fmt::Printf("%5v|%-5v|%05d\n"_gs, p, p, p);
    fmt::Printf("%+v %+v\n"_gs, gp_anon_A{S<Point>({Point{1, 2}})}, S<Point>({Point{3, 4}}));
    fmt::Printf("%+d %+v\n"_gs, p, M<gp::string, Point>({{"k"_gs, Point{5, 6}}}));
    fmt::Println(gp_anon_SN{"s"_gs, nullptr});
    fmt::Printf("%v %+v\n"_gs, Sizes{-1, 2, 3, S<int64_t>({4})}, Sizes{});
    fmt::Println(S<Point>({Point{1, 2}, Point{3, 4}}), gp::array<Point, 1>{Point{5, 6}});
}

struct gp_anon_P {
    gp::ptr<int64_t> P;
    static constexpr const char* gp_type_name = "struct { P *int }";
    static constexpr auto gp_fields = gp::fields(gp::field(&gp_anon_P::P, "P", true, false));
};

void pointers_case() {
    gp::ptr<Point> p = gp::new_<Point>(Point{1, 2});
    fmt::Println(p, gp::new_<gp::slice<int64_t>>(S<int64_t>({1, 2})),
                 gp::new_<gp::array<gp::string, 2>>(gp::array<gp::string, 2>{"a"_gs, "b"_gs}),
                 gp::new_<gp::map<gp::string, int64_t>>(M<gp::string, int64_t>({{"a"_gs, 1}})));
    fmt::Printf("%v %+v %d %s\n"_gs, p, p, p, p);
    gp::ptr<Point> np;
    gp::ptr<int64_t> ni;
    fmt::Println(np, ni, S<gp::ptr<Point>>({nullptr}), gp_anon_P{nullptr});
    fmt::Printf("%v|%d|%s|%t|%5v|%-6v|\n"_gs, np, ni, ni, np, ni, np);
    fmt::Printf("%s %v\n"_gs, S<gp::ptr<Point>>({nullptr}),
                M<gp::string, gp::ptr<int64_t>>({{"k"_gs, nullptr}}));
    gp::ptr<int64_t> x = gp::new_<int64_t>(5);
    gp::string s = fmt::Sprint(x);
    fmt::Println(gp::sub(s, 0, 2), gp::len(s) > 2, s == fmt::Sprintf("%v"_gs, x));
    gp::ptr<gp::ptr<Point>> pp = gp::new_<gp::ptr<Point>>(p);
    s = fmt::Sprint(pp);
    fmt::Println(gp::sub(s, 0, 2), gp::len(s) > 2);
    s = fmt::Sprint(S<gp::ptr<int64_t>>({x}));
    fmt::Println(gp::sub(s, 0, 3), gp::sub(s, gp::len(s) - 1));
    Mixed withp;
    withp.P = p;
    s = fmt::Sprint(gp::new_<Mixed>(withp));
    fmt::Println(gp::sub(s, 0, 10));
    gp::ptr<Mixed> m = gp::new_<Mixed>();
    m->Name = "m"_gs;
    fmt::Printf("%v\n%+v\n"_gs, m, m);
    Named nn;
    fmt::Println(nn);
    gp::ptr<IDs> ids = gp::new_<IDs>(IDs(S<int64_t>({3, 4})));
    fmt::Println(ids, gp::new_<gp::slice<Point>>(S<Point>({Point{1, 2}})));
    fmt::Printf("%s|%d\n"_gs, gp::new_<Point>(Point{7, 8}),
                gp::new_<gp::slice<int64_t>>(S<int64_t>({9})));
    fmt::Printf("%d\n"_gs, ni);
}

void methods_case() {
    Celsius c(21.5);
    fmt::Println(c, S<Celsius>({Celsius(1), Celsius(2.25)}),
                 M<gp::string, Celsius>({{"t"_gs, Celsius(-4)}}));
    fmt::Printf("%v|%s|%d|%q|%8s|%-8v|%.3s|%f|%t\n"_gs, c, c, c, c, c, c, c, c, c);
    fmt::Println(Color(0), Color(1), Color(5), S<Color>({Color(1), Color(0)}),
                 M<Color, int64_t>({{Color(1), 10}, {Color(0), 20}}));
    fmt::Printf("%d %v %s\n"_gs, Color(1), Color(1), S<Color>({Color(0)}));
    gp::ptr<PtrS> ps = gp::new_<PtrS>(PtrS{3}); // boxed: &ps is taken
    fmt::Println(*ps, ps, S<gp::ptr<PtrS>>({ps, nullptr}), S<PtrS>({PtrS{4}}));
    gp::ptr<PtrS> nps;
    fmt::Println(nps);
    fmt::Printf("%v %s %d\n"_gs, nps, ps, ps);
    gp::ptr<Deref> nd;
    fmt::Println(nd, gp::new_<Deref>(Deref{2}), S<gp::ptr<Deref>>({nullptr}));
    gp::ptr<Celsius> nc;
    fmt::Println(nc, S<gp::ptr<Celsius>>({nullptr}));
    fmt::Println(Both{}, S<Both>({Both{}}), gp::new_<Both>());
    fmt::Printf("%v %s %q %d\n"_gs, Both{}, Both{}, Both{}, Both{});
    Wrap w{Celsius(1), Celsius(2), S<Color>({Color(0), Color(1)}), S<Color>({Color(1), Color(0)})};
    fmt::Println(w);
    fmt::Printf("%+v\n"_gs, w);
    fmt::Println(Outer{Color(1), 5}, S<Outer>({Outer{}}));
    fmt::Printf("%v %d\n"_gs, Outer{}, Outer{Color(1), 2});
    fmt::Print(c, Color(1), "\n"_gs);
    fmt::Print(Name("a"_gs), Name("b"_gs), INT64_C(1), INT64_C(2), Name("c"_gs), "\n"_gs);
    fmt::Println(fmt::Sprint(c), fmt::Sprintln(c, Color(0)) == "21.5\xc2\xb0"
                                                               "C red\n"_gs);
    fmt::Println(Stamp(1), later);
    edited = M<int64_t, MapEdit>({{1, MapEdit(1)}, {2, MapEdit(2)}, {3, MapEdit(3)}});
    {
        auto gp_t0 = edited;
        auto gp_t1 = gp::len(edited);
        fmt::Println(gp_t0, gp_t1);
    }
    gp::map<gp::string, Celsius> nm;
    fmt::Println(nm, M<Celsius, gp::string>({{Celsius(2), "two"_gs}, {Celsius(1), "one"_gs}}));
}

struct gp_anon_D {
    gp::ptr<Deref> D;
    static constexpr const char* gp_type_name = "struct { D *main.Deref }";
    static constexpr auto gp_fields = gp::fields(gp::field(&gp_anon_D::D, "D", true, false));
};

void panics_case() {
    fmt::Println(Boom{0}, Boom{1});
    fmt::Println(S<Boom>({Boom{0}, Boom{1}, Boom{2}}));
    fmt::Printf("%s|%v|%d|%q|%5v\n"_gs, Boom{1}, Boom{2}, Boom{1}, Boom{1}, Boom{1});
    fmt::Println(Boom{3});
    fmt::Println(M<gp::string, Boom>({{"a"_gs, Boom{1}}, {"b"_gs, Boom{0}}}));
    fmt::Println(gp::new_<Deref>(Deref{3}), gp_anon_D{nullptr});
    gp::ptr<Deref> nd;
    fmt::Printf("%v|%s|%8v|\n"_gs, nd, nd, nd);
    gp::panic("end"_gs);
}

void verbs_case() {
    int64_t n = 42;
    fmt::Printf("%d|%5d|%-5d|%05d|%+d|% d|%+05d|%-05d|% 05d\n"_gs, n, n, n, n, n, n, n, n, n);
    fmt::Printf("%d|%5d|%-5d|%05d|%+d|% d|%+05d|%-05d\n"_gs, -n, -n, -n, -n, -n, -n, -n, -n);
    fmt::Printf("%.3d|%8.3d|%-8.3d|%08.3d|%.0d|%5.0d|%.0d|%+.0d|\n"_gs, INT64_C(7), INT64_C(7),
                INT64_C(7), INT64_C(7), INT64_C(0), INT64_C(0), INT64_C(1), INT64_C(0));
    fmt::Printf("%v|%+v|%6v|%-6v|%06v|%+6v\n"_gs, n, n, n, n, n, -n);
    fmt::Printf("%d %d %d %d\n"_gs, int8_t(-128), typed<tag::uint64>(UINT64_MAX),
                typed<tag::int64>(INT64_MIN), uint8_t(255));
    fmt::Printf("%020d|%-20d|%+.30d\n"_gs, typed<tag::int64>(INT64_MIN),
                typed<tag::uint64>(UINT64_MAX), INT64_C(1));
    fmt::Printf("%s|%10s|%-10s|%.2s|%5.1s|%-5.3s|%010s|%.0s|%.10s\n"_gs, "go"_gs, "go"_gs, "go"_gs,
                "gopher"_gs, "gopher"_gs, "gopher"_gs, "go"_gs, "go"_gs, "go"_gs);
    fmt::Printf("%8s|%-8s|%.3s|%3s|%.1s|\n"_gs, "h\xc3\xa9llo"_gs,
                "\xe6\x97\xa5\xe6\x9c\xac\xe8\xaa\x9e"_gs,
                "\xe6\x97\xa5\xe6\x9c\xac\xe8\xaa\x9ex"_gs, "\xe2\x98\xba"_gs,
                "\xff"
                "ab"_gs);
    fmt::Printf("%v|%5v|%-5v|%.1v|%q\n"_gs, "s"_gs, "s"_gs, "s"_gs, "st"_gs, "s"_gs);
    fmt::Printf("%t|%v|%6t|%-6t|%06t|%.2t|%q\n"_gs, true, false, true, false, true, true, true);
    fmt::Printf("%f|%.0f|%.1f|%.2f|%8.2f|%-8.2f|%08.2f|%+.2f|% .2f|%+08.2f\n"_gs, pi, 2.5, 0.05,
                2.675, pi, pi, -1.5, 1.5, 1.5, 1.5);
    fmt::Printf("%f|%.3f|%.10f|%f|%.0f|%.0f|%.0f\n"_gs, 1e20, 1e-10, 1.0 / 3, 0.0, 0.5, 1.5, -2.5);
    fmt::Printf("%05f|%+f|% f|%-6f|%6f|%+06f|% 6f\n"_gs, nan, inf, nan, -inf, inf, -inf, inf);
    fmt::Printf("%v|%8v|%-8v|%08v|%+v|% v|%+08v\n"_gs, 3.5, 3.5, 3.5, -3.5, 3.5, 3.5, -3.5);
    fmt::Printf("%.3v|%.3v|%.2v|%.3v|%10.4v|%.0v|%.1v|%.20v\n"_gs, pi, 1234567.0, 100.0, 0.0, e,
                2.5, 0.05, 0.1);
    fmt::Printf("%.3v|%.5v|%.2v|%.4v|%v|%.3v\n"_gs, 1e-5, 123456.0, 0.000123, 1e21, 0.1f, 2.5f);
    fmt::Printf("%.1f|%.2f|%.3f|%5.1f\n"_gs, 0.15f, 2.675f, 1.0f / 3, -0.05f);
    fmt::Printf("%v %d %s\n"_gs, int32_t('x'), int32_t('x'), int32_t('x'));
    fmt::Printf("%%|%5%|%-5%|\n"_gs);
    fmt::Printf("%T %T %T %T %T %T\n"_gs, INT64_C(1), "s"_gs, 1.5, 1.0f, true, int8_t(1));
    fmt::Printf("%T %T %T %T %T\n"_gs, gp::slice_of<int64_t>({}),
                gp::make_map<gp::string, gp::slice<bool>>(), gp::new_<Point>(),
                gp::array<uint16_t, 3>{}, Celsius(1));
    fmt::Printf("%T|%10T|%-10T|\n"_gs, Point{}, INT64_C(1), "s"_gs);
    fmt::Printf("%[2]d %[1]d|%[1]d %d|%[3]s|%[0]d|%[x]d|%[1]\n"_gs, INT64_C(1), INT64_C(2));
    fmt::Printf("%*d|%-*d|%*d|%.*f|%*.*f|\n"_gs, INT64_C(5), INT64_C(1), INT64_C(4), INT64_C(2),
                INT64_C(-4), INT64_C(3), INT64_C(2), pi, INT64_C(8), INT64_C(3), e);
    fmt::Printf("%*d|%.*d|%*d\n"_gs, "x"_gs, INT64_C(1), "y"_gs, INT64_C(2), INT64_C(10000000),
                INT64_C(3));
    fmt::Printf("%[2]*[1]d|%[3]*.[2]*[1]f|\n"_gs, INT64_C(12), INT64_C(2), INT64_C(6));
    fmt::Printf("%-+5d|%+-5d|%0-5d|%-05d|% +d|%+ d\n"_gs, INT64_C(1), INT64_C(2), INT64_C(3),
                INT64_C(4), INT64_C(5), INT64_C(6));
    fmt::Printf("%#v %#d %#q %#s\n"_gs, INT64_C(5), INT64_C(5), "a"_gs, "b"_gs);
    fmt::Printf("%#q %#q %#q\n"_gs, "a`b"_gs, "tab\there"_gs, "nl\n"_gs);
}

void quote_case() {
    for (const gp::string& s : {""_gs,
                                "plain"_gs,
                                "a\"b"_gs,
                                "a\\b"_gs,
                                "tab\t"_gs,
                                "nl\n"_gs,
                                "\x00\x01\x7f"_gs,
                                "\a\b\f\r\v"_gs,
                                "\xc3\xa9"_gs,
                                "\xe6\x97\xa5\xe6\x9c\xac"_gs,
                                "\xc2\xa0"_gs,
                                "\xc2\xad"_gs,
                                "\xe2\x80\xa8"_gs,
                                "\xe2\x80\x8b"_gs,
                                "\xef\xbb\xbf"_gs,
                                "\xe2\x98\xba"_gs,
                                "\xf0\x9f\x98\x80"_gs,
                                "\xf4\x8f\xbf\xbf"_gs,
                                "\xff"_gs,
                                "a\xc0\x80"
                                "b"_gs,
                                "\xe2\x82"_gs,
                                "\xed\xa0\x80"_gs,
                                "\xf4\x90\x80\x80"_gs,
                                "'"_gs}) {
        fmt::Printf("%q|%+q|%#q|%-12q|%12q|%.2q\n"_gs, s, s, s, s, s, s);
    }
    for (int32_t r :
         {int32_t('x'), int32_t('\n'), int32_t('\''), int32_t('"'), int32_t('\\'), 0, 0x7f, 0xa0,
          0xad, 0x2028, 0x263A, 0x1F600, 0x10FFFF, -1, 0xD800, 0x110000, INT32_MAX}) {
        fmt::Printf("%q|%+q|%5q|%-5q|%v\n"_gs, r, r, r, r, r);
    }
    fmt::Printf("%q %q %q %q\n"_gs, uint8_t('a'), typed<tag::int64>(INT64_C(65)),
                typed<tag::uint64>(UINT64_MAX), int8_t(-1));
    fmt::Printf("%q %+q\n"_gs, S<gp::string>({"\xc3\xa9"_gs, "\x00"_gs}),
                S<gp::string>({"\xc3\xa9"_gs}));
}

struct gp_anon_AB {
    double A = 0;
    double B = 0;
    static constexpr const char* gp_type_name = "struct { A float64; B float64 }";
    static constexpr auto gp_fields = gp::fields(gp::field(&gp_anon_AB::A, "A", true, false),
                                                 gp::field(&gp_anon_AB::B, "B", true, false));
};

struct gp_anon_F {
    float F32 = 0;
    double F64 = 0;
    static constexpr const char* gp_type_name = "struct { F32 float32; F64 float64 }";
    static constexpr auto gp_fields = gp::fields(gp::field(&gp_anon_F::F32, "F32", true, false),
                                                 gp::field(&gp_anon_F::F64, "F64", true, false));
};

void floats_case() {
    double a = 0.1, b = 0.2;
    gp::slice<double> f = S<double>(
        {0.1, 0.2, a + b, 1e21, 1e20, 123456789.0, 1e-5, 1e-4, 5e-324, 1.7976931348623157e308, nan,
         -inf, inf, std::copysign(0.0, -1.0), 100, 1e6, 999999.9, 12345.678, 2.5e-7});
    fmt::Println(f);
    fmt::Printf("%v\n%.2f\n%8.3v\n"_gs, f, gp::sub(f, 0, 5), gp::sub(f, 0, 5));
    gp::slice<float> g = S<float>(
        {0.1f, 16777216.0f, 3.4028235e38f, 1e-45f, 1.0f / 3, -0.25f, static_cast<float>(nan)});
    fmt::Println(g);
    fmt::Printf("%v|%.3f|%6.2v\n"_gs, g, gp::sub(g, 0, 3), gp::sub(g, 0, 2));
    fmt::Println(M<double, float>({{0.5, 0.1f}, {-0.5, 1e10f}}), gp_anon_AB{1e-7, 7e7});
    fmt::Printf("%+v|%5.1f\n"_gs, gp_anon_F{0.1f, 0.1}, gp_anon_AB{1.25, -9.75});
    fmt::Println(1e21f, 1e20f, 123456789.0f, 0.000012345f);
    for (double x : {1.0, 12.0, 123.0, 1234.0, 12345.0, 123456.0, 1234567.0, 0.1, 0.01, 0.001,
                     0.0001, 0.00001}) {
        fmt::Printf("%v %.2v %.3v %.4v %.6v %.10v|"_gs, x, x, x, x, x, x);
    }
    fmt::Println();
}

void typed_case() {
    int64_t i64 = 5;
    uint64_t u64 = 6;
    uint64_t up = 7;
    fmt::Printf("%s|%s|%s|%t\n"_gs, typed<tag::int64>(i64), typed<tag::uint64>(u64),
                typed<tag::uintptr>(up), INT64_C(8));
    fmt::Printf("%s %t\n"_gs, typed<tag::slice<tag::int64>>(S<int64_t>({1, 2})),
                typed<tag::map<tag::none, tag::uint64>>(M<gp::string, uint64_t>({{"k"_gs, 3}})));
    fmt::Printf("%s %s\n"_gs, typed<tag::array<tag::uintptr>>(gp::array<uint64_t, 2>{4, 5}),
                typed<tag::map<tag::uint64, tag::slice<tag::int64>>>(
                    M<uint64_t, gp::slice<int64_t>>({{9, S<int64_t>({10})}})));
    gp::ptr<int64_t> pi64;
    fmt::Printf("%s %T %T %T %T\n"_gs, typed<tag::ptr<tag::int64>>(pi64),
                typed<tag::ptr<tag::int64>>(pi64),
                typed<tag::slice<tag::uintptr>>(gp::slice_of<uint64_t>({})),
                typed<tag::map<tag::int64, tag::uint64>>(gp::make_map<int64_t, uint64_t>()),
                typed<tag::array<tag::int64>>(gp::array<int64_t, 1>{}));
    fmt::Printf("%s\n"_gs, Sizes{1, 2, 3, S<int64_t>({4})});
    fmt::Printf("%d\n"_gs, INT64_C(1), typed<tag::int64>(i64), typed<tag::uint64>(u64),
                typed<tag::uintptr>(up), typed<tag::slice<tag::int64>>(S<int64_t>({1})), INT64_C(2),
                uint64_t(3));
    fmt::Printf("%T %T %T %T %T\n"_gs, typed<tag::int64>(i64), typed<tag::uint64>(u64),
                typed<tag::uintptr>(up), INT64_C(1), uint64_t(1));
    fmt::Println(typed<tag::int64>(i64), typed<tag::uint64>(u64), typed<tag::uintptr>(up),
                 typed<tag::slice<tag::int64>>(S<int64_t>({-1})),
                 typed<tag::map<tag::uintptr, tag::int64>>(M<uint64_t, int64_t>({{1, 2}})));
}

void sprint_case() {
    gp::string s = fmt::Sprint("a"_gs, INT64_C(1), INT64_C(2), "b"_gs, INT64_C(3), true, "c"_gs,
                               4.5, Name("n"_gs), INT64_C(5), Name("m"_gs));
    fmt::Println(s, gp::len(s));
    s = fmt::Sprintln("a"_gs, INT64_C(1), S<int64_t>({2}), Point{3, 4});
    fmt::Print(s);
    fmt::Println(gp::len(s));
    s = fmt::Sprintf("%d-%s-%v"_gs, INT64_C(1), "two"_gs, S<double>({3}));
    fmt::Println(s, gp::len(s));
    fmt::Println(fmt::Sprint(), fmt::Sprintln() == "\n"_gs, fmt::Sprintf(""_gs) == ""_gs,
                 fmt::Sprintf("x"_gs) == "x"_gs);
    fmt::Print(S<gp::string>({"a"_gs}), S<gp::string>({"b"_gs}), "\n"_gs);
    fmt::Print("x"_gs, S<int64_t>({1}), INT64_C(2), "\n"_gs);
    fmt::Print(INT64_C(1), INT64_C(2), M<gp::string, int64_t>({}), Point{}, "\n"_gs);
    fmt::Println(fmt::Sprint(Name("x"_gs), Name("y"_gs)),
                 fmt::Sprint(IDs(S<int64_t>({1})), IDs(S<int64_t>({2}))));
    fmt::Print();
    fmt::Println();
}

struct gp_anon_Aint {
    int64_t A = 0;
    static constexpr const char* gp_type_name = "struct { A int }";
    static constexpr auto gp_fields = gp::fields(gp::field(&gp_anon_Aint::A, "A", true, false));
};

void errors_case() {
    fmt::Printf("%d %s %t %f\n"_gs, "x"_gs, INT64_C(5), INT64_C(1), INT64_C(7));
    fmt::Printf("%v %d\n"_gs, INT64_C(1));
    fmt::Printf("%d\n"_gs, INT64_C(1), INT64_C(2), "x"_gs, 2.5, true, S<int64_t>({3}), Point{1, 2},
                Celsius(3), Color(1), gp::new_<Point>(Point{4, 5}));
    fmt::Printf("100%% %z %!\n"_gs, INT64_C(3));
    fmt::Printf("%\303\251|%\xff|%z\n"_gs, INT64_C(1), INT64_C(2), Point{1, 2});
    fmt::Printf("%t %v\n"_gs, "s"_gs, nan);
    fmt::Printf("%s %d %t\n"_gs, true, 1.5, 0.5f);
    fmt::Printf("%s %s %s %s\n"_gs, int8_t(1), uint16_t(2), int32_t(3), uint32_t(4));
    fmt::Printf("%5s|%-6d|%+s|% s|%05t\n"_gs, INT64_C(3), "x"_gs, INT64_C(4), INT64_C(5), "y"_gs);
    fmt::Printf("%s|%d|%t\n"_gs, Celsius(1.5), "str"_gs, Color(1));
    fmt::Printf("%t %s\n"_gs, M<gp::string, bool>({{"k"_gs, true}}), gp_anon_Aint{1});
    fmt::Printf("%w %w\n"_gs, INT64_C(1), Point{1, 2});
    fmt::Printf("no verbs\n"_gs);
    fmt::Printf("%d %d %d\n"_gs, INT64_C(1), INT64_C(2));
    fmt::Printf("%.d|%5.d|%.f|%-.v\n"_gs, INT64_C(0), INT64_C(1), 2.5, INT64_C(3));
    fmt::Printf("end%"_gs);
    fmt::Println();
    fmt::Printf("%.12345678d|%12345678d|%123456789d|x\n"_gs, INT64_C(1), INT64_C(2), INT64_C(3));
}

void big_case() {
    gp::slice<int64_t> s = gp::make_slice<int64_t>(100000);
    for (int64_t i = 0; i < gp::len(s); i++) {
        s[i] = i * 7919 % 100003;
    }
    fmt::Println(s);
    gp::map<int64_t, int64_t> m = gp::make_map<int64_t, int64_t>();
    for (int64_t i = 0; i < 10000; i++) {
        gp::set(m, i * 31 % 10007, i);
    }
    fmt::Println(m);
    gp::slice<double> f = gp::make_slice<double>(20000);
    for (int64_t i = 0; i < gp::len(f); i++) {
        f[i] = static_cast<double>(i) / 7;
    }
    fmt::Println(f);
    gp::slice<gp::string> strs = gp::make_slice<gp::string>(10000);
    for (int64_t i = 0; i < gp::len(strs); i++) {
        strs[i] = itoa(i * i);
    }
    fmt::Printf("%q\n"_gs, strs);
}

// fnv is hash/fnv's 64-bit FNV-1a.
struct fnv {
    uint64_t h = 14695981039346656037u;
    void write(const gp::string& s) {
        for (char c : s.gp_view()) {
            h ^= static_cast<uint8_t>(c);
            h *= 1099511628211u;
        }
    }
};

void runes_case() {
    fnv h;
    int64_t n = 0;
    for (int32_t r = -2; r <= 0x110001; r++) {
        gp::string s = gp::rune_to_string(r);
        h.write(fmt::Sprintf("%q%+q%q%+q"_gs, s, s, r, r));
        n++;
    }
    fmt::Println(n, typed<tag::uint64>(h.h));
    h = fnv{};
    for (int b = 0; b < 256; b++) {
        for (int c = 0; c < 256; c += 15) {
            char raw[3] = {static_cast<char>(b), static_cast<char>(c), 'x'};
            gp::string s = gp::string::gp_copy(std::string_view(raw, 3));
            h.write(fmt::Sprintf("%q%+q"_gs, s, s));
        }
    }
    fmt::Println(typed<tag::uint64>(h.h));
}

void mutate_case() {
    held = Holder{S<Clear>({Clear(1), Clear(2), Clear(3)})};
    fmt::Println(held, held.L == nullptr);
    held = Holder{S<Clear>({Clear(1), Clear(2), Clear(3)})};
    fmt::Println(gp::static_ptr(held), held.L == nullptr);
    drops = S<gp::slice<Drop>>({S<Drop>({Drop(1), Drop(2)}), S<Drop>({Drop(3)})});
    fmt::Println(gp::static_ptr(drops), drops == nullptr);
    drops = S<gp::slice<Drop>>({S<Drop>({Drop(1), Drop(2)}), S<Drop>({Drop(3)})});
    fmt::Println(drops, drops == nullptr);
}

// The read-only rules of reflect for embedded fields: emb is unexported and embedded in Pub,
// Pub is exported and embedded in the unexported hid, Top embeds hid and *pemb.
struct Pub2 {
    Celsius D;
    static constexpr const char* gp_type_name = "main.Pub2";
    static constexpr auto gp_fields = gp::fields(gp::field(&Pub2::D, "D", true, false));
};

struct emb {
    Celsius C;
    Celsius c;
    gp::slice<Celsius> L;
    gp::map<gp::string, Celsius> M;
    Pub2 P;
    Pub2 p;
    static constexpr const char* gp_type_name = "main.emb";
    static constexpr auto gp_fields =
        gp::fields(gp::field(&emb::C, "C", true, false), gp::field(&emb::c, "c", false, false),
                   gp::field(&emb::L, "L", true, false), gp::field(&emb::M, "M", true, false),
                   gp::field(&emb::P, "P", true, false), gp::field(&emb::p, "p", false, false));
};

struct Pub {
    emb emb_; // embedded
    Celsius E;
    static constexpr const char* gp_type_name = "main.Pub";
    static constexpr auto gp_fields =
        gp::fields(gp::field(&Pub::emb_, "emb", false, true), gp::field(&Pub::E, "E", true, false));
};

struct hid {
    Pub Pub_; // embedded
    Celsius F;
    Celsius g;
    static constexpr const char* gp_type_name = "main.hid";
    static constexpr auto gp_fields =
        gp::fields(gp::field(&hid::Pub_, "Pub", true, true), gp::field(&hid::F, "F", true, false),
                   gp::field(&hid::g, "g", false, false));
};

struct pemb {
    Celsius H;
    static constexpr const char* gp_type_name = "main.pemb";
    static constexpr auto gp_fields = gp::fields(gp::field(&pemb::H, "H", true, false));
};

struct Top {
    hid hid_;            // embedded
    gp::ptr<pemb> pemb_; // embedded *pemb
    gp::slice<Pub> Pubs;
    gp::slice<Pub> pubs;
    Pub x;
    static constexpr const char* gp_type_name = "main.Top";
    static constexpr auto gp_fields = gp::fields(
        gp::field(&Top::hid_, "hid", false, true), gp::field(&Top::pemb_, "pemb", false, true),
        gp::field(&Top::Pubs, "Pubs", true, false), gp::field(&Top::pubs, "pubs", false, false),
        gp::field(&Top::x, "x", false, false));
};

struct gp_anon_emb {
    emb emb_;
    static constexpr const char* gp_type_name = "struct { main.emb }";
    static constexpr auto gp_fields = gp::fields(gp::field(&gp_anon_emb::emb_, "emb", false, true));
};

struct gp_anon_xemb {
    emb x;
    static constexpr const char* gp_type_name = "struct { x main.emb }";
    static constexpr auto gp_fields = gp::fields(gp::field(&gp_anon_xemb::x, "x", false, false));
};

void embedded_case() {
    emb e{Celsius(1),
          Celsius(2),
          S<Celsius>({Celsius(3)}),
          M<gp::string, Celsius>({{"k"_gs, Celsius(4)}}),
          Pub2{Celsius(5)},
          Pub2{Celsius(6)}};
    Pub pub{e, Celsius(7)};
    fmt::Println(pub);
    fmt::Printf("%+v\n%v\n%s\n%d\n"_gs, pub, gp::new_<Pub>(pub), S<Pub>({pub}), pub);
    fmt::Println(M<gp::string, Pub>({{"m"_gs, pub}}), gp::array<Pub, 1>{pub});
    hid h{pub, Celsius(8), Celsius(9)};
    fmt::Println(h);
    fmt::Printf("%+v\n"_gs, h);
    Top t{h, nullptr, S<Pub>({pub}), S<Pub>({pub}), pub};
    fmt::Println(t);
    fmt::Printf("%+v\n"_gs, gp::new_<Top>(t));
    fmt::Println(S<Top>({t}), M<int64_t, Top>({{1, t}}));
    fmt::Println(e, gp_anon_emb{e}, gp_anon_xemb{e});
}

void main() {
    std::string_view c = gp::args()[1].gp_view();
    if (c == "fmt-slices") {
        slices_case();
    } else if (c == "fmt-maps") {
        maps_case();
    } else if (c == "fmt-structs") {
        structs_case();
    } else if (c == "fmt-pointers") {
        pointers_case();
    } else if (c == "fmt-methods") {
        methods_case();
    } else if (c == "fmt-panics") {
        panics_case();
    } else if (c == "fmt-verbs") {
        verbs_case();
    } else if (c == "fmt-quote") {
        quote_case();
    } else if (c == "fmt-floats") {
        floats_case();
    } else if (c == "fmt-typed") {
        typed_case();
    } else if (c == "fmt-sprint") {
        sprint_case();
    } else if (c == "fmt-errors") {
        errors_case();
    } else if (c == "fmt-big") {
        big_case();
    } else if (c == "fmt-runes") {
        runes_case();
    } else if (c == "fmt-mutate") {
        mutate_case();
    } else if (c == "fmt-embedded") {
        embedded_case();
    } else {
        fmt::Println("unknown case"_gs, gp::args()[1]);
        gp::os::exit(3);
    }
}

} // namespace gpkg_main

int main(int argc, char** argv) {
    return gp::run_main(argc, argv, nullptr, &gpkg_main::main);
}
