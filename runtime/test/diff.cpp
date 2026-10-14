// The C++ side of the runtime's differential test: the program of
// runtime/testdata/diff/main.go, written against the runtime as gocpp would generate it (Go's
// int is int64_t, a rune is int32_t, a byte is uint8_t). runtime/cxx_test.go runs both with
// each case name and compares the exit codes and the merged stdout and stderr.
#include <gocpp/rt.hpp>
#include <gocpp/os.hpp>

#include <bit>
#include <cmath>
#include <cstdint>
#include <limits>
#include <memory>
#include <string_view>
#include <vector>

namespace gpkg_main {

namespace fmt = gp::fmtshim;

const double inf = std::numeric_limits<double>::infinity();
const double nan = std::numeric_limits<double>::quiet_NaN();

void println_case() {
    int64_t* p = nullptr;
    gp::println(true, false, INT64_C(-5), int8_t(-128), UINT64_MAX, INT64_MIN, "str"_gs,
                int32_t('x'), ""_gs);
    gp::println(0.0, std::copysign(0.0, -1.0), inf, -inf, nan, 1e300, 5e-324, 1e21);
    gp::println(123456789.0, 0.1f, 1.5f, 0.1, 2.5e-10, 1.7976931348623157e308, 9.9999999);
    gp::println(p);
    gp::println();
    gp::println(1.5, 3.0, -0.25, int32_t('x'));
    gp::print("a"_gs, INT64_C(1), INT64_C(2), "b"_gs, true, 1.5, "\n"_gs);
    gp::print();
    gp::print("\n"_gs);
    gp::println(uint8_t(255), uint16_t(65535), uint32_t(4294967295u), int16_t(-32768),
                int32_t(INT32_MIN));
}

void fmt_case() {
    fmt::Println(123456.0, 1234567.0, 1000000.0, 1e20, 1e21, 0.0001, 0.00001, 3.0, 2.5, 0.1f,
                 16777216.0f, 1e-5f);
    fmt::Println(inf, -inf, nan, std::copysign(0.0, -1.0), 0.0, 5e-324, 1.7976931348623157e308,
                 -1.5e-7, 100000.0, 999999.0, 3.4028235e38f);
    fmt::Println("x"_gs, INT64_C(1), true, int8_t(-3), UINT64_MAX, int32_t('a'), uint8_t(7));
    fmt::Println();
    fmt::Print("a"_gs, "b"_gs, INT64_C(1), INT64_C(2), true, "c"_gs, 3.5, "\n"_gs);
    fmt::Print(INT64_C(1), INT64_C(2), "\n"_gs);
    fmt::Print("a"_gs, INT64_C(1), INT64_C(2), "b"_gs, "\n"_gs);
    fmt::Print();
    fmt::Print(true, false, 1.5, 2.5f, "\n"_gs);
    fmt::Printf("%v|%d|%s|%t|%f|%v|%f|%f|%v\n"_gs, 1.5, INT64_C(-42), "s"_gs, true, 3.14159265,
                "str"_gs, 1e300, -inf, int8_t(5));
    fmt::Printf("%f %f %f %f %f\n"_gs, 0.0, 1e-7, 2.5, 0.1f, std::copysign(0.0, -1.0));
    fmt::Printf("%d %s %t %f\n"_gs, "x"_gs, INT64_C(5), INT64_C(1), INT64_C(7));
    fmt::Printf("%v %d\n"_gs, INT64_C(1));
    fmt::Printf("%d\n"_gs, INT64_C(1), INT64_C(2));
    fmt::Printf("%d\n"_gs, INT64_C(1), INT64_C(2), "x"_gs, 2.5, true);
    fmt::Printf("100%% %z %\n"_gs, INT64_C(3));
    fmt::Printf("%v %v\n"_gs, true, int32_t('x'));
    fmt::Printf("%d %d %v\n"_gs, uint8_t(200), UINT64_MAX, 0.1f);
    fmt::Printf("%s %d %t\n"_gs, true, 1.5, 0.5f);
    fmt::Printf("%s %s %s %s\n"_gs, int8_t(1), uint16_t(2), int32_t(3), uint32_t(4));
    fmt::Printf("%!\n"_gs);
    fmt::Printf("%\303\251\n"_gs, INT64_C(1));
    fmt::Printf("%t %v\n"_gs, "s"_gs, nan);
    fmt::Printf("%f %f %f\n"_gs, nan, inf, -2.5);
    fmt::Printf("%v %v\n"_gs, std::copysign(0.0, -1.0), 0.0f);
    fmt::Printf("no verbs\n"_gs);
    fmt::Printf("end%"_gs);
    fmt::Println();
}

void floats_case() {
    uint64_t x = UINT64_C(0x9E3779B97F4A7C15);
    const double scales[] = {1e-10, 1e-5, 1e-4, 1e-3, 0.1, 1, 10, 1e5, 1e6, 1e7, 1e20, 1e21, 1e22};
    const uint64_t nscales = sizeof scales / sizeof scales[0];
    for (int64_t i = 0; i < 1500; i++) {
        x = x * UINT64_C(6364136223846793005) + UINT64_C(1442695040888963407);
        double f = std::bit_cast<double>(x);
        float g = std::bit_cast<float>(static_cast<uint32_t>(x >> 32));
        double h = static_cast<double>(static_cast<int64_t>(x >> 11) % 2000001 - 1000000) / 1000;
        double k = h / 7 * scales[x % nscales];
        float k32 = static_cast<float>(k);
        fmt::Println(f, g, h, k, k32);
        gp::println(f, g, h, k, k32);
        fmt::Printf("%f %f %f %v\n"_gs, f, g, k, k32);
    }
}

const int64_t counts[] = {
    0, 1, 2, 7, 8, 9, 15, 16, 17, 31, 32, 33, 63, 64, 65, 200, INT64_C(1) << 40};

template <typename T>
void table(gp::string name, const std::vector<T>& vals) {
    for (T a : vals) {
        for (T b : vals) {
            if (b != 0) {
                fmt::Println(name, a, "/"_gs, b, "="_gs, gp::div(a, b), gp::rem(a, b));
            }
        }
    }
    for (T a : vals) {
        for (int64_t n : counts) {
            fmt::Println(name, a, "<<>>"_gs, n, "="_gs, gp::shl(a, n), gp::shr(a, n),
                         gp::shl(a, static_cast<uint8_t>(n)), gp::shr(a, static_cast<uint64_t>(n)));
        }
    }
}

void arith_case() {
    table<int8_t>("int8"_gs,
                  {INT8_MIN, INT8_MIN + 1, -7, -2, -1, 0, 1, 2, 3, 7, INT8_MAX - 1, INT8_MAX});
    table<int16_t>("int16"_gs, {INT16_MIN, INT16_MIN + 1, -7, -1, 0, 1, 2, 7, INT16_MAX});
    table<int32_t>("int32"_gs, {INT32_MIN, INT32_MIN + 1, -7, -1, 0, 1, 2, 7, INT32_MAX});
    table<int64_t>("int64"_gs, {INT64_MIN, INT64_MIN + 1, -7, -1, 0, 1, 2, 7, INT64_MAX});
    table<uint8_t>("uint8"_gs, {0, 1, 2, 3, 7, 128, UINT8_MAX - 1, UINT8_MAX});
    table<uint16_t>("uint16"_gs, {0, 1, 2, 7, 1 << 15, UINT16_MAX});
    table<uint32_t>("uint32"_gs, {0, 1, 2, 7, UINT32_C(1) << 31, UINT32_MAX});
    table<uint64_t>("uint64"_gs, {0, 1, 2, 7, UINT64_C(1) << 63, UINT64_MAX});
}

const double f2i_vals[] = {0,
                           -0.5,
                           0.9,
                           -1,
                           1.5,
                           127.9,
                           128,
                           -129,
                           255.5,
                           256,
                           300,
                           -300,
                           32768,
                           65536 + 7,
                           -32769,
                           2147483647,
                           2147483648,
                           -2147483648,
                           -2147483648.9,
                           -2147483649,
                           3e9 + 7,
                           4294967296 + 9,
                           -3e9,
                           9.2233720368547748e18,
                           9223372036854775808.0,
                           -9223372036854775808.0,
                           -9.223372036854777e18,
                           1.8446744073709550e19,
                           18446744073709551616.0,
                           1e300,
                           -1e300,
                           inf,
                           -inf,
                           nan};

void f2i_case() {
    for (double f : f2i_vals) {
        fmt::Println(f, gp::f2i<int64_t>(f), gp::f2i<int32_t>(f), gp::f2i<int16_t>(f),
                     gp::f2i<int8_t>(f), gp::f2i<uint64_t>(f), gp::f2i<uint32_t>(f),
                     gp::f2i<uint16_t>(f), gp::f2i<uint8_t>(f));
    }
    for (double f64 : f2i_vals) {
        float f = static_cast<float>(f64);
        fmt::Println(f, gp::f2i<int64_t>(f), gp::f2i<int32_t>(f), gp::f2i<int16_t>(f),
                     gp::f2i<int8_t>(f), gp::f2i<uint64_t>(f), gp::f2i<uint32_t>(f),
                     gp::f2i<uint16_t>(f), gp::f2i<uint8_t>(f));
    }
}

void strings_case() {
    gp::string a = "abc"_gs, b = "abd"_gs, e = ""_gs;
    gp::string s = a + b;
    fmt::Println(s, gp::len(s), s[0], s[5], gp::len(e), gp::len(a + e), e + e == e);
    fmt::Println(a<b, a == b, a != b, a <= b, a> b, a >= b, e<a, "\377"_gs> a, "ab"_gs < a,
                 s + e == s);
    gp::string t = "x"_gs;
    for (int64_t i = 0; i < 3; i++) {
        t += "yz"_gs;
    }
    fmt::Println(t, gp::len(t), t[gp::len(t) - 1]);
    gp::println(s, gp::len(s), s[2], t);
    fmt::Printf("%s|%v|%d\n"_gs, s, t, gp::len(s));
    fmt::Println("tab\tnl\\n bytes:\303\251"_gs, gp::len("\303\251"_gs), "a\000b"_gs == "a\000b"_gs,
                 gp::len("a\000b"_gs));
}

struct point {
    int64_t X = 0;
    int64_t Y = 0;
    int64_t Sum() const;
};

int64_t point::Sum() const {
    point p = *this;
    return p.X + p.Y;
}

struct other {
    int64_t X = 0;
    int64_t Y = 0;
};

point globalPoint;
int64_t globalArr[3];

gp::ptr<int64_t> newCounter() {
    gp::ptr<int64_t> c = gp::new_<int64_t>(INT64_C(1)); // boxed: its address is returned
    return c;
}

void ptr_case() {
    gp::ptr<int64_t> p = gp::new_<int64_t>();
    fmt::Println(*p, p == nullptr, p != nullptr);
    *p = 5;
    gp::ptr<int64_t> q = p;
    *q += 2;
    fmt::Println(*p, *q, p == q);
    gp::ptr<int64_t> r = gp::new_<int64_t>();
    *r = 7;
    fmt::Println(*r == *p, r == p);
    gp::ptr<int64_t> n;
    fmt::Println(n == nullptr, n == gp::ptr<int64_t>(nullptr), n == r);

    gp::ptr<int64_t> c = newCounter(), d = newCounter();
    (*c)++;
    fmt::Println(*c, *d, c == d);

    gp::ptr<point> pt = gp::new_<point>(point{.X = 1, .Y = 2});
    gp::ptr<int64_t> py = gp::ptr<int64_t>(pt, &pt->Y);
    *py = 7;
    fmt::Println(pt->X, pt->Y, *py, pt->Sum(), py == gp::ptr<int64_t>(pt, &pt->Y),
                 gp::ptr<int64_t>(pt, &pt->X) == gp::ptr<int64_t>(pt, &pt->Y));
    pt = nullptr;
    *py = 8;
    fmt::Println(*py, pt == nullptr);

    gp::ptr<point> g = gp::static_ptr(globalPoint);
    g->X = 3;
    fmt::Println(globalPoint.X, gp::static_ptr(globalPoint) == g,
                 gp::static_ptr(globalPoint.X) == gp::ptr<int64_t>(g, &g->X),
                 gp::static_ptr(globalPoint.X) == gp::static_ptr(globalPoint.Y));
    gp::ptr<int64_t> e = gp::static_ptr(globalArr[1]);
    *e = 9;
    fmt::Println(globalArr[1], e == gp::static_ptr(globalArr[1]),
                 e == gp::static_ptr(globalArr[2]));

    gp::ptr<other> o = gp::ptr_cast<other>(g);
    o->Y = 11;
    fmt::Println(globalPoint.Y, gp::ptr_cast<point>(o) == g);

    gp::ptr<point> pt2 = gp::new_<point>(point{.X = 4, .Y = 5});
    gp::weak_ptr<point> w = gp::weak_make(pt2);
    fmt::Println(gp::weak_value(w) == pt2, w == gp::weak_make(pt2), gp::weak_value(w)->X,
                 w == gp::weak_make(gp::new_<point>(point{.X = 4, .Y = 5})));
    gp::weak_ptr<int64_t> wy = gp::weak_make(gp::ptr<int64_t>(pt2, &pt2->Y));
    fmt::Println(*gp::weak_value(wy), gp::weak_value(wy) == gp::ptr<int64_t>(pt2, &pt2->Y));
    gp::weak_ptr<point> wg = gp::weak_make(gp::static_ptr(globalPoint));
    fmt::Println(gp::weak_value(wg) == gp::static_ptr(globalPoint), wg == gp::weak_make(g),
                 gp::weak_value(wg)->X);
    gp::weak_ptr<point> z;
    fmt::Println(gp::weak_value(z) == nullptr, z == gp::weak_make(gp::ptr<point>(nullptr)), z == w);
}

// big_string is 1 MiB of lines: much more than a pipe holds.
gp::string big_string() {
    gp::string line = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcde\n"_gs;
    gp::string s = line;
    while (gp::len(s) < 1 << 20) {
        s = s + s;
    }
    return s;
}

void substr_case() {
    gp::string s = "hello, world"_gs;
    int64_t i = 7, j = 12;
    fmt::Println(gp::sub(s, i, j), gp::sub(s, i), gp::sub(s, 0, 5), s, gp::sub(s, j),
                 gp::len(gp::sub(s, 3, 3)), gp::sub(s, 1, 4) == "ell"_gs,
                 gp::sub(gp::sub(s, i, j), 1, 3));
    gp::string t = gp::concat(gp::sub(s, 0, 5), "!"_gs, gp::sub(s, i), "?"_gs);
    fmt::Println(t, gp::len(t), t[5], gp::sub(s, 0, 0) == ""_gs);
    gp::string u = ""_gs;
    for (int64_t k = 0; k < 5; k++) {
        u = gp::concat(u, gp::sub(s, k, k + 1), "-"_gs);
    }
    fmt::Println(u, gp::len(u));
}

void show_runes(gp::string s) {
    for (auto it = gp::range_string(s); gp::range_next(it);) {
        int64_t i = it.gp_i;
        int32_t r = it.gp_r;
        fmt::Printf("%d:%d "_gs, i, r);
    }
    fmt::Println(gp::len(s));
}

void show_string(int32_t r) {
    gp::string s = gp::rune_to_string(r);
    fmt::Printf("%d:"_gs, gp::len(s));
    for (int64_t i = 0; i < gp::len(s); i++) {
        fmt::Printf(" %d"_gs, s[i]);
    }
    fmt::Println();
}

void utf8_case() {
    show_runes(""_gs);
    show_runes("abc"_gs);
    show_runes("h\303\251llo, \344\270\226\347\225\214 \360\237\230\200"_gs);
    show_runes("a\377b\300\200c"_gs);
    show_runes("\342\202 \355\240\200 \364\220\200\200 \360\237\230"_gs);
    show_runes("\302\200\337\277\340\240\200\357\277\277\360\220\200\200\364\217\277\277"_gs);
    for (auto it = gp::range_string("A\303\251\342\202\254\360\237\230\200"_gs);
         gp::range_next(it);) {
        show_string(it.gp_r);
    }
    show_string(0);
    show_string(-1);
    show_string(0xD800);
    show_string(0xDFFF);
    show_string(0x10FFFF);
    show_string(0x110000);
    show_string(INT32_MIN);
}

gp::string the_case;

// The "leak" case drops the only pointer to a block through this variable, so the compiler
// can't remove the allocation.
int64_t* volatile leak_sink;

bool slice_case(std::string_view c);

void main() {
    std::string_view c = the_case.gp_view();
    if (slice_case(c)) {
        return;
    }
    if (c == "println") {
        println_case();
    } else if (c == "fmt") {
        fmt_case();
    } else if (c == "floats") {
        floats_case();
    } else if (c == "arith") {
        arith_case();
    } else if (c == "f2i") {
        f2i_case();
    } else if (c == "strings") {
        strings_case();
    } else if (c == "substr") {
        substr_case();
    } else if (c == "utf8") {
        utf8_case();
    } else if (c == "substr-alen") {
        gp::string s = "abc"_gs;
        int64_t i = 1, j = 5;
        fmt::Println(gp::sub(s, i, j));
    } else if (c == "substr-b") {
        gp::string s = "abc"_gs;
        int64_t i = 3, j = 2;
        fmt::Println(gp::sub(s, i, j));
    } else if (c == "substr-neg") {
        gp::string s = "abc"_gs;
        int64_t i = -1;
        fmt::Println(gp::sub(s, i));
    } else if (c == "substr-from") {
        gp::string s = "abc"_gs;
        uint8_t i = 4;
        fmt::Println(gp::sub(s, i));
    } else if (c == "order") {
        fmt::Print("a\n"_gs);
        gp::println("b"_gs);
        fmt::Println("c"_gs);
        gp::print("d\n"_gs);
        fmt::Printf("e%d\n"_gs, INT64_C(1));
        fmt::Print("no newline, "_gs);
        gp::println("then stderr"_gs);
        gp::panic("p"_gs);
    } else if (c == "panic-str") {
        fmt::Println("before"_gs);
        gp::panic("boom"_gs);
    } else if (c == "panic-multi") {
        gp::panic("a\nb\nc"_gs);
    } else if (c == "panic-empty") {
        gp::panic(""_gs);
    } else if (c == "panic-int") {
        gp::panic(INT64_C(42));
    } else if (c == "panic-float") {
        gp::panic(1.5);
    } else if (c == "panic-bool") {
        gp::panic(true);
    } else if (c == "div") {
        int64_t z = 0;
        fmt::Println(gp::div(INT64_C(10), z));
    } else if (c == "rem") {
        int8_t z = 0;
        fmt::Println(gp::rem(int8_t(10), z));
    } else if (c == "shl") {
        int64_t n = -1;
        fmt::Println(gp::shl(INT64_C(1), n));
    } else if (c == "shr") {
        int8_t n = -5;
        fmt::Println(gp::shr(uint8_t(1), n));
    } else if (c == "index") {
        gp::string s = "abc"_gs;
        int64_t i = 5;
        fmt::Println(s[i]);
    } else if (c == "negindex") {
        gp::string s = "abc"_gs;
        int64_t i = -1;
        fmt::Println(s[i]);
    } else if (c == "uindex") {
        gp::string s = "abc"_gs;
        uint64_t i = UINT64_MAX;
        fmt::Println(s[i]);
    } else if (c == "ptr") {
        ptr_case();
    } else if (c == "nil-deref") {
        gp::ptr<int64_t> p;
        fmt::Println(*p);
    } else if (c == "nil-store") {
        gp::ptr<int64_t> p;
        *p = 1;
    } else if (c == "nil-field") {
        gp::ptr<point> p;
        fmt::Println(p->X);
    } else if (c == "nil-method") {
        gp::ptr<point> p;
        fmt::Println(p->Sum());
    } else if (c == "nil-addr") {
        gp::ptr<point> p;
        gp::ptr<int64_t> q = gp::ptr<int64_t>(p, &p->Y);
        fmt::Println(q == nullptr);
    } else if (c == "exit0") {
        fmt::Print("only stdout\n"_gs);
    } else if (c == "big") {
        fmt::Print(big_string());
        gp::println("done"_gs);
    } else if (c == "bigerr") {
        gp::println(big_string());
        gp::println("still alive"_gs);
        gp::os::exit(3);
    } else if (c == "bad-owner") {
        // Not in the Go program: a non-nil pointer without an owner breaks the runtime's
        // invariant (ptr.hpp), and the check ends the program like a Go fatal error.
        int64_t x = 0;
        gp::ptr<int64_t> p(std::shared_ptr<void>(), &x);
        fmt::Println(*p);
    } else if (c == "leak") {
        // Not in the Go program: LeakSanitizer must report this in the asan mode.
        leak_sink = new int64_t[100];
        leak_sink = nullptr;
        fmt::Println("leaked"_gs);
    }
}

void init() {
    if (gp::args().size() > 1) {
        the_case = gp::args()[1];
    }
}

// --- slices (Phase 03 step 3.5a) ---

using ints = gp::slice<int64_t>;
using bytes = gp::slice<uint8_t>;

void dump(const gp::string& name, const ints& s) {
    fmt::Print(name, " "_gs, gp::len(s), " ["_gs);
    for (int64_t i = 0; i < gp::len(s); i++) {
        if (i > 0) {
            fmt::Print(" "_gs);
        }
        fmt::Print(s[i]);
    }
    fmt::Print("]\n"_gs);
}

void dump_bytes(const gp::string& name, const bytes& s) {
    fmt::Print(name, " "_gs, gp::len(s), " ["_gs);
    for (int64_t i = 0; i < gp::len(s); i++) {
        if (i > 0) {
            fmt::Print(" "_gs);
        }
        fmt::Print(s[i]);
    }
    fmt::Print("]\n"_gs);
}

void slices_case() {
    // nil and empty slices
    ints n;
    fmt::Println(n == nullptr, gp::len(n), gp::cap(n), gp::sub(n, 0, 0) == nullptr,
                 gp::len(gp::sub(n, 0)), gp::sub(n, 0) == nullptr);
    ints e = gp::slice_of<int64_t>({});
    fmt::Println(e == nullptr, gp::len(e), gp::cap(e), gp::sub(e, 0, 0) == nullptr);
    ints m0 = gp::make_slice<int64_t>(0);
    fmt::Println(m0 == nullptr, gp::len(m0), gp::cap(m0), gp::sub(m0, 0, 0, 0) == nullptr);

    // make: zeroed, with the exact len and cap; sub-slices have Go's exact len and cap
    ints m = gp::make_slice<int64_t>(3, 10);
    dump("make"_gs, m);
    fmt::Println(gp::len(m), gp::cap(m), gp::len(gp::sub(m, 0, 10)), gp::cap(gp::sub(m, 2)),
                 gp::len(gp::sub(m, 2, 5, 7)), gp::cap(gp::sub(m, 2, 5, 7)),
                 gp::cap(gp::sub(m, 10, 10)), gp::len(gp::sub(m, 3)), gp::cap(gp::sub(m, 3)));
    m = gp::sub(m, 0, 10);
    dump("make10"_gs, m);
    ints m1 = gp::make_slice<int64_t>(4);
    fmt::Println(gp::len(m1), gp::cap(m1));

    // two slices of one array see each other's writes
    ints a = gp::slice_of<int64_t>({1, 2, 3, 4, 5});
    fmt::Println(gp::len(a), gp::cap(a));
    ints b = gp::sub(a, 1, 3);
    b[0] = 20;
    dump("a"_gs, a);
    dump("b"_gs, b);
    fmt::Println(gp::len(b), gp::cap(b));
    b = gp::append(b, INT64_C(30));
    dump("a"_gs, a);
    dump("b"_gs, b);
    ints c = gp::sub(a, 1, 3, 3);
    c = gp::append(c, INT64_C(99));
    c[0] = -1;
    dump("a"_gs, a);
    dump("c"_gs, c);
    fmt::Println(gp::cap(c) >= gp::len(c));

    // append forms
    ints x;
    x = gp::append(x);
    fmt::Println(x == nullptr);
    x = gp::append(x, INT64_C(1));
    x = gp::append(x, INT64_C(2), INT64_C(3), INT64_C(4));
    x = gp::append_slice(x, x);
    dump("x"_gs, x);
    x = gp::append_slice(gp::sub(x, 0, 1), gp::sub(x, 2));
    dump("x"_gs, x);
    ints y = gp::slice_of<int64_t>({1, 2, 3, 4});
    y = gp::append(gp::sub(y, 0, 1), y[2], y[1]);
    dump("y"_gs, y);
    ints z = gp::make_slice<int64_t>(2, 8);
    z[0] = 7;
    z[1] = 8;
    z = gp::append_slice(z, z);
    z = gp::append_slice(z, z);
    dump("z"_gs, z);
    ints w = gp::make_slice<int64_t>(5, 10);
    for (int64_t i = 0; i < gp::len(w); i++) {
        w[i] = i;
    }
    ints w2 = gp::append_slice(gp::sub(w, 0, 1), gp::sub(w, 0, 4));
    dump("w"_gs, w);
    dump("w2"_gs, w2);
    ints nn;
    nn = gp::append_slice(nn, gp::slice_of<int64_t>({}));
    fmt::Println(nn == nullptr);
    nn = gp::append_slice(nn, ints());
    fmt::Println(nn == nullptr);
    nn = gp::append_slice(nn, gp::slice_of<int64_t>({6, 7}));
    dump("nn"_gs, nn);

    // copy: the number copied; overlap safe
    ints k = gp::slice_of<int64_t>({1, 2, 3, 4, 5});
    fmt::Println(gp::copy(gp::sub(k, 1), k));
    dump("k"_gs, k);
    k = gp::slice_of<int64_t>({1, 2, 3, 4, 5});
    fmt::Println(gp::copy(k, gp::sub(k, 2)));
    dump("k"_gs, k);
    int64_t n3 = gp::copy(k, gp::slice_of<int64_t>({}));
    int64_t n4 = gp::copy(ints(), k);
    fmt::Println(n3, n4, gp::copy(gp::sub(k, 0, 2), gp::slice_of<int64_t>({8, 9, 10})));
    dump("k"_gs, k);

    // []byte and strings
    bytes bs = gp::slice_of<uint8_t>({'h', 'i'});
    bs = gp::append_slice(bs, "llo"_gs);
    bs = gp::append_slice(bs, ""_gs);
    dump_bytes("bs"_gs, bs);
    bytes nb = gp::make_slice<uint8_t>(3);
    // In Go's order: C++ evaluates the arguments of a call in any order.
    int64_t n1 = gp::copy(nb, "abcdef"_gs);
    int64_t n2 = gp::copy(gp::sub(nb, 2), "z"_gs);
    fmt::Println(n1, n2, gp::copy(nb, ""_gs));
    dump_bytes("nb"_gs, nb);
    bytes nilb;
    nilb = gp::append_slice(nilb, ""_gs);
    fmt::Println(nilb == nullptr);
    nilb = gp::append_slice(nilb, "go"_gs);
    dump_bytes("nilb"_gs, nilb);

    // clear
    ints cl = gp::slice_of<int64_t>({1, 2, 3, 4});
    gp::clear(gp::sub(cl, 1, 3));
    dump("cl"_gs, cl);
    ints cln;
    gp::clear(cln);

    // strings as elements
    gp::slice<gp::string> ss = gp::slice_of<gp::string>({"a"_gs, "b"_gs});
    ss = gp::append(ss, "c"_gs, "d"_gs, "e"_gs);
    gp::slice<gp::string> t = gp::sub(ss, 1, 4);
    t[0] = "B"_gs;
    gp::clear(gp::sub(ss, 3));
    fmt::Println(gp::len(ss), ss[0], ss[1], ss[2], ss[3] == ""_gs, ss[4] == ""_gs, t[0],
                 gp::len(t));

    // nested slices
    gp::slice<ints> grid = gp::make_slice<ints>(3);
    for (int64_t i = 0; i < gp::len(grid); i++) {
        grid[i] = gp::make_slice<int64_t>(i + 1);
        grid[i][i] = i * 10;
    }
    grid = gp::append(grid, grid[1]);
    grid[3][0] = 55;
    fmt::Println(gp::len(grid), grid[1][0], grid[2][2], gp::len(grid[3]), grid[0] == nullptr);

    // pointers to elements keep the array alive
    ints pe = gp::slice_of<int64_t>({1, 2, 3});
    gp::ptr<int64_t> p = gp::ptr<int64_t>(pe.gp_owner(), &pe[1]);
    *p = 22;
    fmt::Println(pe[1]);
    pe = nullptr;
    *p += 1;
    fmt::Println(*p, pe == nullptr);
    ints pe2 = gp::slice_of<int64_t>({4, 5});
    gp::ptr<int64_t> q = gp::ptr<int64_t>(pe2.gp_owner(), &pe2[0]);
    pe2 = gp::append(pe2, INT64_C(6));
    *q = 40;
    fmt::Println(pe2[0], *q, gp::len(pe2));

    // index types
    ints it = gp::slice_of<int64_t>({10, 20, 30});
    uint8_t u8 = 2;
    int8_t i8 = 1;
    uint64_t u64 = 0;
    fmt::Println(it[u8], it[i8], it[u64], gp::len(gp::sub(it, u8)), gp::len(gp::sub(it, i8, u8)),
                 gp::cap(gp::sub(it, i8, u8, u8)));

    // slice to array: the length check passes (the array type comes in step 3.6)
    gp::check_convert(2, gp::len(it));
    fmt::Println(it[0], it[1]);
}

void slice_grow_case() {
    ints s;
    bool ok = true;
    int64_t reallocs = 0;
    for (int64_t i = 0; i < 1000000; i++) {
        int64_t before = gp::cap(s);
        s = gp::append(s, i);
        if (gp::cap(s) < gp::len(s)) {
            ok = false;
        }
        if (gp::cap(s) != before) {
            reallocs++;
        }
    }
    int64_t sum = 0;
    for (int64_t i = 0; i < gp::len(s); i++) {
        sum += s[i];
    }
    fmt::Println(gp::len(s), sum, ok, s[999999], reallocs < 100);
    bytes bs;
    for (int64_t i = 0; i < 100000; i++) {
        bs = gp::append(bs, static_cast<uint8_t>(i));
    }
    fmt::Println(gp::len(bs), bs[99999], gp::cap(bs) >= gp::len(bs));
    gp::slice<gp::string> ss;
    for (int64_t i = 0; i < 10000; i++) {
        ss = gp::append(ss, "x"_gs);
    }
    fmt::Println(gp::len(ss), ss[9999], gp::cap(ss) >= gp::len(ss));
    ints t = gp::make_slice<int64_t>(0, 3);
    t = gp::append(t, INT64_C(1), INT64_C(2), INT64_C(3), INT64_C(4), INT64_C(5), INT64_C(6),
                   INT64_C(7));
    fmt::Println(gp::len(t), gp::cap(t) >= 7, t[6]);
    fmt::Println(gp::cap(gp::make_slice<int64_t>(5)), gp::cap(gp::make_slice<int64_t>(0, 7)),
                 gp::cap(gp::slice_of<int64_t>({1, 2, 3})));
}

struct node {
    gp::slice<node> kids;
    int64_t v = 0;
};

void node_vals(const gp::slice<node>& q) {
    fmt::Print(gp::len(q), " ["_gs);
    for (int64_t i = 0; i < gp::len(q); i++) {
        if (i > 0) {
            fmt::Print(" "_gs);
        }
        fmt::Print(q[i].v);
    }
    fmt::Print("]\n"_gs);
}

// Elements whose source array is owned only by the element that the store overwrites
// (q[0].kids): the store must not free the source while it reads it (ASan checks).
void slice_free_case() {
    using nodes = gp::slice<node>;
    nodes q =
        gp::slice_of<node>({node{gp::slice_of<node>({node{{}, 5}, node{{}, 6}, node{{}, 7}}), 0}});
    q = gp::append_slice(gp::sub(q, 0, 0), q[0].kids);
    node_vals(q);
    q = gp::slice_of<node>({node{gp::slice_of<node>({node{{}, 1}}), 0}, node{{}, 5}});
    fmt::Println(gp::copy(gp::sub(q, 0, 1), q[0].kids));
    node_vals(q);
    q = gp::slice_of<node>(
        {node{gp::slice_of<node>({node{{}, 3}, node{{}, 5}, node{{}, 6}, node{{}, 7}}), 0},
         node{{}, 1}, node{{}, 2}});
    fmt::Println(gp::copy(q, q[0].kids));
    node_vals(q);
    q = gp::slice_of<node>({node{gp::slice_of<node>({node{{}, 5}, node{{}, 8}}), 0}});
    q = gp::append(gp::sub(q, 0, 0), q[0].kids[0]);
    node_vals(q);
    q = gp::slice_of<node>({node{gp::slice_of<node>({node{{}, 5}, node{{}, 8}}), 0}});
    q = gp::append(gp::sub(q, 0, 0), q[0].kids[1], q[0].kids[0]);
    node_vals(q);
    nodes k = gp::slice_of<node>({node{{}, 9}});
    k[0].kids = k;
    gp::clear(k[0].kids);
    fmt::Println(k[0].v, k[0].kids == nullptr);
}

bool slice_case(std::string_view c) {
    ints s3 = gp::make_slice<int64_t>(3, 5);
    if (c == "slices") {
        slices_case();
    } else if (c == "slice-grow") {
        slice_grow_case();
    } else if (c == "slice-free") {
        slice_free_case();
    } else if (c == "slice-index") {
        ints s = gp::make_slice<int64_t>(3);
        int64_t i = 5;
        fmt::Println(s[i]);
    } else if (c == "slice-index-neg") {
        ints s = gp::make_slice<int64_t>(3);
        int64_t i = -1;
        fmt::Println(s[i]);
    } else if (c == "slice-index-u8") {
        ints s = gp::make_slice<int64_t>(3);
        uint8_t i = 200;
        fmt::Println(s[i]);
    } else if (c == "slice-index-nil") {
        ints s;
        int64_t i = 0;
        fmt::Println(s[i]);
    } else if (c == "slice-hi") {
        int64_t h = 6;
        fmt::Println(gp::len(gp::sub(s3, 0, h)));
    } else if (c == "slice-hi-neg") {
        int64_t h = -1;
        fmt::Println(gp::len(gp::sub(s3, 0, h)));
    } else if (c == "slice-hi-u") {
        uint64_t h = UINT64_MAX;
        fmt::Println(gp::len(gp::sub(s3, 0, h)));
    } else if (c == "slice-lo") {
        int64_t l = 3, h = 2;
        fmt::Println(gp::len(gp::sub(s3, l, h)));
    } else if (c == "slice-lo-neg") {
        int64_t l = -2;
        fmt::Println(gp::len(gp::sub(s3, l)));
    } else if (c == "slice-from") {
        int64_t l = 4;
        fmt::Println(gp::len(gp::sub(s3, l)));
    } else if (c == "slice-nil-sub") {
        ints s;
        int64_t h = 1;
        fmt::Println(gp::len(gp::sub(s, 0, h)));
    } else if (c == "slice3-max") {
        int64_t m = 6;
        fmt::Println(gp::len(gp::sub(s3, 0, 1, m)));
    } else if (c == "slice3-max-neg") {
        int64_t m = -1;
        fmt::Println(gp::len(gp::sub(s3, 0, 1, m)));
    } else if (c == "slice3-max-u") {
        uint64_t m = UINT64_MAX;
        fmt::Println(gp::len(gp::sub(s3, 0, 1, m)));
    } else if (c == "slice3-hi") {
        int64_t h = 4, m = 2;
        fmt::Println(gp::len(gp::sub(s3, 0, h, m)));
    } else if (c == "slice3-hi-neg") {
        int64_t h = -1, m = 2;
        fmt::Println(gp::len(gp::sub(s3, 0, h, m)));
    } else if (c == "slice3-lo") {
        int64_t l = 3, h = 2, m = 4;
        fmt::Println(gp::len(gp::sub(s3, l, h, m)));
    } else if (c == "slice3-lo-neg") {
        int64_t l = -1, h = 2, m = 4;
        fmt::Println(gp::len(gp::sub(s3, l, h, m)));
    } else if (c == "make-len") {
        int64_t n = -1;
        fmt::Println(gp::len(gp::make_slice<int64_t>(n)));
    } else if (c == "make-len-big") {
        int64_t n = INT64_C(1) << 60;
        fmt::Println(gp::len(gp::make_slice<int64_t>(n)));
    } else if (c == "make-cap") {
        int64_t n = 2, cp = 1;
        fmt::Println(gp::len(gp::make_slice<int64_t>(n, cp)));
    } else if (c == "make-cap-big") {
        int64_t n = 1, cp = INT64_C(1) << 60;
        fmt::Println(gp::len(gp::make_slice<int64_t>(n, cp)));
    } else if (c == "slice-convert") {
        ints s = gp::slice_of<int64_t>({1, 2});
        gp::check_convert(3, gp::len(s));
        fmt::Println(s[0]);
    } else {
        return false;
    }
    return true;
}

} // namespace gpkg_main

int main(int argc, char** argv) {
    return gp::run_main(argc, argv, &gpkg_main::init, &gpkg_main::main);
}
