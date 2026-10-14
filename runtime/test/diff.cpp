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

gp::string the_case;

// The "leak" case drops the only pointer to a block through this variable, so the compiler
// can't remove the allocation.
int64_t* volatile leak_sink;

void main() {
    std::string_view c = the_case.gp_view();
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

} // namespace gpkg_main

int main(int argc, char** argv) {
    return gp::run_main(argc, argv, &gpkg_main::init, &gpkg_main::main);
}
