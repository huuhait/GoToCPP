// The unit tests of fmt (gocpp/fmt.hpp, runtime/src/fmt.cpp, ADR-028) for what the differential
// tests (tests/run/fmt_*.gp, compared with real Go) can't reach from Go code: the descriptors that
// the emitter writes, written by hand here (a named struct with a gp_full, the field offsets,
// the operations of gocpp/typeops.hpp), the methods found in the method tables, the plain
// operands, the State of a Format method, Errorf's types, a method that changes what prints,
// weak pointers, addresses, and the speed of a big slice. runtime/cxx_test.go compiles this file
// with the runtime's .cpp files, with g++ and clang++, at -O0, at -O2 and with ASan + UBSan, and
// runs it like test/unit.cpp. It prints "ok: N checks" and exits 0, or prints each failed check
// and exits 1.
#include "threads_ended.hpp"

#include <gocpp/rt.hpp>
#include <gocpp/os.hpp>

#include <algorithm>
#include <atomic>
#include <chrono>
#include <cstddef>
#include <cstdint>
#include <cstdio>
#include <string>
#include <thread>
#include <vector>

namespace fmt = gp::fmt;

namespace gpkg_main {

struct Point {
    int64_t X = 0;
    int64_t Y = 0;
};

// A struct with an unexported field, a slice, a map, a pointer and an interface.
struct Rec {
    gp::string Name;
    int64_t age = 0;
    gp::slice<int64_t> Tags;
    gp::map<gp::string, int64_t> M;
    gp::ptr<Point> P;
    gp::any A;
};

// A named string type with String, a struct with an Error method on the pointer, a Formatter,
// a type whose String panics, one whose String changes the slice that holds it.
struct Name : gp::newtype<Name, gp::string> {
    using gp::newtype<Name, gp::string>::newtype;
};
struct Err {
    gp::string msg;
};
struct Fm {
    int64_t n = 0;
};
struct Boom {
    int64_t unused = 0;
};
struct Mut {
    int64_t n = 0;
};

gp::slice<Mut> mut_slice;
gp::ptr<gp::slice<Mut>> mut_ptr;

// --- the thunks and the method tables (what the emitter writes) ---

gp::string thunk_name_string(const gp::iface& self) {
    return gp::string::gp_copy("<" + std::string(gp::unbox<Name>(self).gp_value().gp_view()) + ">");
}
gp::string thunk_err_error(const gp::iface& self) {
    return gp::unbox<gp::ptr<Err>>(self)->msg;
}
gp::error thunk_err_unwrap(const gp::iface&) {
    return gp::error();
}
void thunk_fm_format(const gp::iface& self, fmt::State f, int32_t verb) {
    auto [w, wok] = f.Width();
    auto [p, pok] = f.Precision();
    std::string s = "Fm{" + std::to_string(gp::unbox<Fm>(self).n) +
                    " verb=" + std::string(1, static_cast<char>(verb)) + " w=" + std::to_string(w) +
                    (wok ? "/set" : "") + " p=" + std::to_string(p) + (pok ? "/set" : "") +
                    (f.Flag('+') ? " plus" : "") + (f.Flag('-') ? " minus" : "") +
                    (f.Flag('#') ? " sharp" : "") + "}";
    f.Write(gp::to_bytes(gp::string::gp_copy(s)));
}
gp::string thunk_boom_string(const gp::iface&) {
    gp::panic(gp::to_iface<gp::any>(1.5, &gp::td_float64));
}
gp::string thunk_boom_ptr_string(const gp::iface&) {
    gp::panic("never"_gs);
}
gp::string thunk_mut_string(const gp::iface& self) {
    // Changes the slice that is being printed, and returns a text.
    mut_slice = gp::slice<Mut>();
    if (mut_ptr != nullptr) {
        *mut_ptr = gp::slice<Mut>();
    }
    return gp::string::gp_copy("mut" + std::to_string(gp::unbox<Mut>(self).n));
}

extern const gp::type_desc td_point;
extern const gp::type_desc td_rec;

const gp::type_desc* full_point() {
    static const gp::field_desc f[] = {
        gp::field_desc("X", nullptr, &gp::td_int, nullptr, false, gp::offset_of(&Point::X)),
        gp::field_desc("Y", nullptr, &gp::td_int, nullptr, false, gp::offset_of(&Point::Y)),
    };
    static const gp::type_desc d = gp::completed(td_point, gp::shape_struct(f, 2));
    return &d;
}
const gp::type_desc td_point = gp::named<Point>(
    "main.Point", "main", gp::shape_lazy(gp::shape_struct(nullptr, 0), &full_point));

const gp::type_desc* td_ptr_point() {
    static const gp::type_desc* const p = gp::td_ptr<Point>(&td_point);
    return p;
}

const gp::type_desc* full_rec() {
    static const gp::field_desc f[] = {
        gp::field_desc("Name", nullptr, &gp::td_string, nullptr, false, gp::offset_of(&Rec::Name)),
        gp::field_desc("age", "main", &gp::td_int, nullptr, false, gp::offset_of(&Rec::age)),
        gp::field_desc("Tags", nullptr, gp::td_slice<int64_t>(&gp::td_int), nullptr, false,
                       gp::offset_of(&Rec::Tags)),
        gp::field_desc("M", nullptr, gp::td_map<gp::string, int64_t>(&gp::td_string, &gp::td_int),
                       nullptr, false, gp::offset_of(&Rec::M)),
        gp::field_desc("P", nullptr, td_ptr_point(), nullptr, false, gp::offset_of(&Rec::P)),
        gp::field_desc("A", nullptr, &gp::td_any, nullptr, false, gp::offset_of(&Rec::A)),
    };
    static const gp::type_desc d = gp::completed(td_rec, gp::shape_struct(f, 6));
    return &d;
}
const gp::type_desc td_rec =
    gp::named<Rec>("main.Rec", "main", gp::shape_lazy(gp::shape_struct(nullptr, 0), &full_rec));

const gp::method name_methods[] = {
    gp::method_of<gp::string()>("String", nullptr, "func() string", &thunk_name_string)};
const gp::type_desc td_name = gp::named<Name>(
    "main.Name", "main", gp::shape_basic(gp::kind::String), gp::methods_of(name_methods));

const gp::method err_pmethods[] = {
    gp::method_of<gp::string()>("Error", nullptr, "func() string", &thunk_err_error),
    gp::method_of<gp::error()>("Unwrap", nullptr, "func() error", &thunk_err_unwrap),
};
const gp::type_desc td_err = gp::named<Err>("main.Err", "main", gp::shape_struct(nullptr, 0), {},
                                            gp::methods_of(err_pmethods));
const gp::type_desc* td_ptr_err() {
    static const gp::type_desc* const p = gp::td_ptr<Err>(&td_err);
    return p;
}

const gp::method fm_methods[] = {gp::method_of<void(fmt::State, int32_t)>(
    "Format", nullptr, "func(fmt.State, int32)", &thunk_fm_format)};
const gp::type_desc td_fm =
    gp::named<Fm>("main.Fm", "main", gp::shape_struct(nullptr, 0), gp::methods_of(fm_methods));

const gp::method boom_methods[] = {
    gp::method_of<gp::string()>("String", nullptr, "func() string", &thunk_boom_string)};
const gp::type_desc td_boom = gp::named<Boom>("main.Boom", "main", gp::shape_struct(nullptr, 0),
                                              gp::methods_of(boom_methods));
const gp::method boom_pmethods[] = {
    gp::method_of<gp::string()>("String", nullptr, "func() string", &thunk_boom_ptr_string)};
const gp::type_desc td_boom_p = gp::named<Boom>("main.BoomP", "main", gp::shape_struct(nullptr, 0),
                                                {}, gp::methods_of(boom_pmethods));
const gp::type_desc* td_ptr_boom() {
    static const gp::type_desc* const p = gp::td_ptr<Boom>(&td_boom_p);
    return p;
}

const gp::method mut_methods[] = {
    gp::method_of<gp::string()>("String", nullptr, "func() string", &thunk_mut_string)};
const gp::type_desc td_mut =
    gp::named<Mut>("main.Mut", "main", gp::shape_struct(nullptr, 0), gp::methods_of(mut_methods));

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

#define CHECK_EQ(got, want)                                                                        \
    do {                                                                                           \
        std::string gp_got = (got);                                                                \
        std::string gp_want = (want);                                                              \
        report(gp_got == gp_want, __FILE__, __LINE__,                                              \
               #got " = \"" + gp_got + "\", want \"" + gp_want + "\"");                            \
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

// The descriptors: the fields of a named struct come from gp_full, with their offsets; the
// operations read slices, maps and pointers.
void test_descriptors() {
    CHECK(td_point.gp_nfields == 0 && td_point.gp_full != nullptr);
    const gp::type_desc* f = gp::full(&td_point);
    CHECK(f != &td_point && f->gp_nfields == 2);
    CHECK(f->gp_fields[1].gp_offset == offsetof(Point, Y));
    CHECK(f->gp_name == td_point.gp_name && f->gp_full == nullptr);
    CHECK(gp::full(f) == f);
    CHECK(gp::full(&gp::td_int) == &gp::td_int);
    const gp::type_desc* r = gp::full(&td_rec);
    CHECK(r->gp_nfields == 6 && r->gp_fields[1].gp_pkg != nullptr &&
          r->gp_fields[0].gp_pkg == nullptr);
    CHECK(r->gp_fields[5].gp_offset == offsetof(Rec, A));
    // Operations.
    gp::slice<int64_t> s = gp::slice_of<int64_t>({4, 5, 6});
    const gp::type_ops* so = gp::td_slice<int64_t>(&gp::td_int)->gp_ops;
    CHECK(so->gp_seq_len(&s) == 3 && so->gp_seq_data(&s) == s.gp_data() && !so->gp_nil(&s));
    gp::slice<int64_t> nils;
    CHECK(so->gp_nil(&nils) && so->gp_seq_len(&nils) == 0);
    gp::map<gp::string, int64_t> m = gp::map_of<gp::string, int64_t>({{"a"_gs, 1}, {"b"_gs, 2}});
    const gp::type_ops* mo = gp::td_map<gp::string, int64_t>(&gp::td_string, &gp::td_int)->gp_ops;
    std::shared_ptr<void> snap = mo->gp_map_snap(&m);
    CHECK(mo->gp_snap_len(snap.get()) == 2);
    gp::ptr<Point> p = gp::new_<Point>(Point{1, 2});
    const gp::type_ops* po = td_ptr_point()->gp_ops;
    CHECK(po->gp_target(&p) == p.gp_get() && po->gp_owner(&p) != nullptr);
    gp::array<int64_t, 3> a{};
    CHECK(gp::td_array<int64_t, 3>(&gp::td_int)->gp_ops->gp_seq_len(&a) == 3);
    CHECK(gp::td_slice<int64_t>(&gp::td_int) == gp::td_slice<int64_t>(&gp::td_int));
}

// Plain operands need no descriptor; the others carry one.
void test_operands() {
    CHECK_EQ(
        str(fmt::Sprint(int64_t(1), "a"_gs, true, 2.5, int8_t(-3), uint8_t(200), int32_t('x'))),
        "1atrue 2.5 -3 200 120");
    CHECK_EQ(str(fmt::Sprintf("%v %T"_gs, int64_t(5), int64_t(5))), "5 int");
    CHECK_EQ(str(fmt::Sprintf("%v %T"_gs, fmt::as(&gp::td_int64, int64_t(5)),
                              fmt::as(&gp::td_int64, int64_t(5)))),
             "5 int64");
    CHECK_EQ(str(fmt::Sprintf("%T %T %T"_gs, fmt::as(&gp::td_uint, uint64_t(1)),
                              fmt::as(&gp::td_uint64, uint64_t(1)),
                              fmt::as(&gp::td_uintptr, uint64_t(1)))),
             "uint uint64 uintptr");
    CHECK_EQ(str(fmt::Sprint(gp::any())), "<nil>");
    CHECK_EQ(str(fmt::Sprint(nullptr, nullptr)), "<nil> <nil>");
    CHECK_EQ(str(fmt::Sprintf("%d|%s"_gs, gp::any(), gp::any())), "%!d(<nil>)|%!s(<nil>)");
    gp::any i = gp::to_iface<gp::any>(int64_t(42), &gp::td_int);
    gp::any s = gp::to_iface<gp::any>("text"_gs, &gp::td_string);
    CHECK_EQ(str(fmt::Sprint(i, s, 1.5)), "42text1.5");
    CHECK_EQ(str(fmt::Sprintln(i, s)), "42 text\n");
    CHECK_EQ(str(fmt::Sprintf("%5d|%-6s|%T %T"_gs, i, s, i, s)), "   42|text  |int string");
    // A final a... is the list of operands.
    gp::slice<gp::any> rest = gp::slice_of<gp::any>({i, s, gp::any()});
    CHECK_EQ(str(fmt::Sprint(fmt::spread(rest))), "42text<nil>");
    CHECK_EQ(str(fmt::Sprintf("%d-%s-%v"_gs, fmt::spread(rest))), "42-text-<nil>");
    CHECK_EQ(str(fmt::Sprint(fmt::spread(gp::slice<gp::any>()))), "");
    CHECK_EQ(str(fmt::Errorf("%v %v"_gs, fmt::spread(rest)).Error()), "42 text%!(EXTRA <nil>)");
    // The big array that the emitter copies to the heap prints as the value.
    gp::ptr<gp::array<int64_t, 3>> big = gp::new_<gp::array<int64_t, 3>>();
    (*big)[1] = 7;
    CHECK_EQ(str(fmt::Sprint(fmt::as(gp::td_array<int64_t, 3>(&gp::td_int), fmt::heap(big)))),
             "[0 7 0]");
}

void test_structs() {
    Rec r;
    r.Name = "ann"_gs;
    r.age = 31;
    r.Tags = gp::slice_of<int64_t>({1, 2});
    r.M = gp::map_of<gp::string, int64_t>({{"z"_gs, 1}, {"a"_gs, 2}});
    r.A = gp::to_iface<gp::any>(Point{3, 4}, &td_point);
    CHECK_EQ(str(fmt::Sprint(fmt::as(&td_rec, r))), "{ann 31 [1 2] map[a:2 z:1] <nil> {3 4}}");
    CHECK_EQ(str(fmt::Sprintf("%+v"_gs, fmt::as(&td_rec, r))),
             "{Name:ann age:31 Tags:[1 2] M:map[a:2 z:1] P:<nil> A:{X:3 Y:4}}");
    CHECK_EQ(str(fmt::Sprintf("%#v"_gs, fmt::as(&td_rec, r))),
             "main.Rec{Name:\"ann\", age:31, Tags:[]int{1, 2}, M:map[string]int{\"a\":2, \"z\":1}, "
             "P:(*main.Point)(nil), A:main.Point{X:3, Y:4}}");
    r.P = gp::new_<Point>(Point{5, 6});
    std::string got = str(fmt::Sprint(fmt::as(&td_rec, r)));
    CHECK_EQ(got, "{ann 31 [1 2] map[a:2 z:1] " + hex(r.P.gp_get()) + " {3 4}}");
    CHECK_EQ(str(fmt::Sprint(fmt::as(td_ptr_point(), r.P))), "&{5 6}");
    CHECK_EQ(str(fmt::Sprintf("%T %T"_gs, fmt::as(&td_rec, r), fmt::as(td_ptr_point(), r.P))),
             "main.Rec *main.Point");
}

// Methods are found in the descriptor's method table: for a value, a value in an interface, an
// element of a slice and a pointer.
void test_methods() {
    Name n = Name(gp::string::gp_copy("x"));
    CHECK_EQ(str(fmt::Sprint(fmt::as(&td_name, n))), "<x>");
    CHECK_EQ(str(fmt::Sprintf("%d %s %q %v %10v|"_gs, fmt::as(&td_name, n), fmt::as(&td_name, n),
                              fmt::as(&td_name, n), fmt::as(&td_name, n), fmt::as(&td_name, n))),
             "%!d(main.Name=x) <x> \"<x>\" <x>        <x>|");
    gp::any a = gp::to_iface<gp::any>(n, &td_name);
    CHECK_EQ(str(fmt::Sprint(a)), "<x>");
    gp::slice<Name> sl = gp::slice_of<Name>({n, Name(gp::string::gp_copy("y"))});
    CHECK_EQ(str(fmt::Sprint(fmt::as(gp::td_slice<Name>(&td_name), sl))), "[<x> <y>]");
    // An error through a pointer.
    gp::ptr<Err> e = gp::new_<Err>(Err{"bad"_gs});
    CHECK_EQ(str(fmt::Sprint(fmt::as(td_ptr_err(), e))), "bad");
    gp::error ie = gp::to_iface<gp::error>(e, td_ptr_err());
    CHECK_EQ(str(fmt::Sprintf("%v|%s|%q|%T"_gs, ie, ie, ie, ie)), "bad|bad|\"bad\"|*main.Err");
}

// A panic in a method, a Formatter, and the State.
void test_panic_and_formatter() {
    CHECK_EQ(str(fmt::Sprint(fmt::as(&td_boom, Boom{}))), "%!v(PANIC=String method: 1.5)");
    CHECK_EQ(str(fmt::Sprintf("%s|%q"_gs, fmt::as(&td_boom, Boom{}), fmt::as(&td_boom, Boom{}))),
             "%!s(PANIC=String method: 1.5)|%!q(PANIC=String method: 1.5)");
    CHECK_EQ(str(fmt::Sprint(fmt::as(td_ptr_boom(), gp::ptr<Boom>()))), "<nil>");
    CHECK_EQ(str(fmt::Sprintf("%d"_gs, fmt::as(&td_boom, Boom{}))), "{}"); // (no fields described)
    CHECK_EQ(str(fmt::Sprintf("%+8.3d"_gs, fmt::as(&td_fm, Fm{3}))),
             "Fm{3 verb=d w=8/set p=3/set plus}");
    CHECK_EQ(str(fmt::Sprintf("%-#x %v"_gs, fmt::as(&td_fm, Fm{1}), fmt::as(&td_fm, Fm{2}))),
             "Fm{1 verb=x w=0 p=0 minus sharp} Fm{2 verb=v w=0 p=0}");
    // %w is for Errorf: an error elsewhere is the error form.
    gp::ptr<Err> e = gp::new_<Err>(Err{"bad"_gs});
    CHECK_EQ(str(fmt::Sprintf("%w"_gs, fmt::as(td_ptr_err(), e))), "%!w(*main.Err=&{})");
}

// Errorf: the types, the messages, the Unwrap protocol through the method tables.
void test_errorf() {
    gp::ptr<Err> e = gp::new_<Err>(Err{"base"_gs});
    gp::error ie = gp::to_iface<gp::error>(e, td_ptr_err());
    gp::error plain = fmt::Errorf("plain %d"_gs, int64_t(1));
    CHECK_EQ(str(plain.Error()), "plain 1");
    CHECK_EQ(str(fmt::Sprintf("%T"_gs, plain)), "*errors.errorString");
    gp::error w = fmt::Errorf("wrap: %w"_gs, ie);
    CHECK_EQ(str(w.Error()), "wrap: base");
    CHECK_EQ(str(fmt::Sprintf("%T"_gs, w)), "*fmt.wrapError");
    const gp::method* unwrap = nullptr;
    for (std::size_t i = 0; i < w.gp_type()->gp_nmethods; i++) {
        if (std::string(w.gp_type()->gp_methods[i].gp_name) == "Unwrap") {
            unwrap = &w.gp_type()->gp_methods[i];
        }
    }
    CHECK(unwrap != nullptr && std::string(unwrap->gp_sig) == "func() error");
    if (unwrap != nullptr) {
        gp::error inner = reinterpret_cast<gp::thunk_t<gp::error()>*>(unwrap->gp_fn)(w);
        CHECK(inner == ie);
    }
    gp::error two = fmt::Errorf("%w and %w"_gs, ie, ie);
    CHECK_EQ(str(fmt::Sprintf("%T %v"_gs, two, two)), "*fmt.wrapErrors base and base");
    gp::error nilw = fmt::Errorf("n: %w"_gs, gp::error());
    CHECK_EQ(str(nilw.Error()), "n: %!w(<nil>)");
    CHECK_EQ(str(fmt::Errorf("%w"_gs, int64_t(3)).Error()), "%!w(int=3)");
}

// While an element prints, the array that holds it is kept alive, so a String method that
// drops the slice can't free it (Go's garbage collector does the same for free).
void test_changed_slice() {
    mut_slice = gp::make_slice<Mut>(3);
    for (int64_t i = 0; i < 3; i++) {
        mut_slice[i].n = i + 1;
    }
    // The operand is a copy of the slice: the method's change can't reach it.
    CHECK_EQ(str(fmt::Sprint(fmt::as(gp::td_slice<Mut>(&td_mut), mut_slice))), "[mut1 mut2 mut3]");
    mut_slice = gp::make_slice<Mut>(2);
    gp::ptr<gp::slice<Mut>> ps = gp::new_<gp::slice<Mut>>(mut_slice);
    mut_slice = gp::slice<Mut>();
    mut_ptr = ps;
    // Through a pointer to the slice: printing &[...] reads the slice again for each element, as
    // reflect does, and the array is held while it prints: the String method of the first element
    // drops the pointer's slice (the last owner of the array) and the element is still there.
    std::string out =
        str(fmt::Sprint(fmt::as(gp::td_ptr<gp::slice<Mut>>(gp::td_slice<Mut>(&td_mut)), ps)));
    CHECK_EQ(out, "&[mut0]");
    mut_ptr = nullptr;
}

// Addresses and %p.
void test_addresses() {
    gp::ptr<int64_t> p = gp::new_<int64_t>(5);
    const gp::type_desc* pt = gp::td_ptr<int64_t>(&gp::td_int);
    CHECK_EQ(str(fmt::Sprint(fmt::as(pt, p))), hex(p.gp_get()));
    CHECK_EQ(str(fmt::Sprintf("%p"_gs, fmt::as(pt, p))), hex(p.gp_get()));
    CHECK_EQ(str(fmt::Sprintf("%d"_gs, fmt::as(pt, p))),
             std::to_string(
                 static_cast<unsigned long long>(reinterpret_cast<std::uintptr_t>(p.gp_get()))));
    CHECK_EQ(str(fmt::Sprint(fmt::as(pt, gp::ptr<int64_t>()))), "<nil>");
    CHECK_EQ(str(fmt::Sprintf("%v %+v %#v"_gs, fmt::as(pt, gp::ptr<int64_t>()),
                              fmt::as(pt, gp::ptr<int64_t>()), fmt::as(pt, gp::ptr<int64_t>()))),
             "<nil> <nil> (*int)(nil)");
    gp::slice<int64_t> s = gp::slice_of<int64_t>({1});
    CHECK_EQ(str(fmt::Sprintf("%p"_gs, fmt::as(gp::td_slice<int64_t>(&gp::td_int), s))),
             hex(s.gp_data()));
}

// A map with pointer keys sorts them by address; weak pointers print as Go's struct.
void test_pointer_keys_and_weak() {
    std::vector<gp::ptr<int64_t>> ps;
    gp::map<gp::ptr<int64_t>, int64_t> m = gp::make_map<gp::ptr<int64_t>, int64_t>();
    for (int i = 0; i < 4; i++) {
        ps.push_back(gp::new_<int64_t>(i));
        gp::set(m, ps.back(), int64_t(i));
    }
    std::sort(ps.begin(), ps.end(), [](const gp::ptr<int64_t>& a, const gp::ptr<int64_t>& b) {
        return a.gp_get() < b.gp_get();
    });
    std::string want = "map[";
    for (std::size_t i = 0; i < ps.size(); i++) {
        want += (i > 0 ? " " : "") + hex(ps[i].gp_get()) + ":" + std::to_string(*ps[i]);
    }
    const gp::type_desc* pt = gp::td_ptr<int64_t>(&gp::td_int);
    CHECK_EQ(str(fmt::Sprint(fmt::as(gp::td_map<gp::ptr<int64_t>, int64_t>(pt, &gp::td_int), m))),
             want + "]");
    gp::weak_ptr<int64_t> z;
    const gp::type_desc* wt = gp::td_weak<int64_t>(&gp::td_int);
    CHECK_EQ(str(fmt::Sprintf("%v|%+v|%T"_gs, fmt::as(wt, z), fmt::as(wt, z), fmt::as(wt, z))),
             "{[] <nil>}|{_:[] u:<nil>}|weak.Pointer[int]");
    gp::weak_ptr<int64_t> w = gp::weak_make(ps[0]);
    CHECK_EQ(str(fmt::Sprint(fmt::as(wt, w))), "{[] " + hex(ps[0].gp_get()) + "}");
    CHECK(wt == gp::td_weak<int64_t>(&gp::td_int));
}

// Sprint gives a string that owns its bytes; a big width or precision works.
void test_sprint() {
    gp::string s = fmt::Sprint(INT64_C(12), "ab"_gs);
    CHECK_EQ(str(s), "12ab");
    CHECK(s.gp_owner() != nullptr);
    CHECK_EQ(str(fmt::Sprint()), "");
    CHECK(gp::len(fmt::Sprintf("%1000000d"_gs, INT64_C(7))) == INT64_C(1000000));
    CHECK(gp::len(fmt::Sprintf("%.1000000d"_gs, INT64_C(7))) == INT64_C(1000000));
    CHECK(gp::len(fmt::Sprintf("%.1000000f"_gs, 0.5)) == INT64_C(1000002));
    CHECK_EQ(str(fmt::Sprintf("%.60v"_gs, 0.1)),
             "0.1000000000000000055511151231257827021181583404541015625");
    CHECK_EQ(str(fmt::Sprintf("%x %X %#x %b %e"_gs, 3.5, 3.5, 3.5, 3.5, 3.5)),
             "0x1.cp+01 0X1.CP+01 0x1.c000p+01 7881299347898368p-51 3.500000e+00");
    gp::slice<uint8_t> b = fmt::Appendf(gp::slice<uint8_t>(), "%d-%s"_gs, INT64_C(1), "x"_gs);
    CHECK_EQ(std::string(reinterpret_cast<const char*>(b.gp_data()), b.gp_len()), "1-x");
}

// Println of a 1e5-element []int, the speed check of the plan (Go: about 2-4 ms): the output
// grows linearly. Reported, not checked (the machines differ), except a generous ratio.
double sprint_ms(int64_t n) {
    gp::slice<int64_t> s = gp::make_slice<int64_t>(n);
    for (int64_t i = 0; i < n; i++) {
        s[i] = i * 7919 % 100003;
    }
    auto t0 = std::chrono::steady_clock::now();
    gp::string out = fmt::Sprintln(fmt::as(gp::td_slice<int64_t>(&gp::td_int), s));
    auto t1 = std::chrono::steady_clock::now();
    CHECK(gp::len(out) > n);
    return std::chrono::duration<double, std::milli>(t1 - t0).count();
}

std::string test_speed() {
    // The best of three of each, and a ratio, not a wall-clock bound: on a loaded machine one run
    // can stall for seconds (ten times the elements should take about ten times as long).
    double small = sprint_ms(100000);
    double big = sprint_ms(1000000);
    for (int i = 0; i < 2; i++) {
        small = std::min(small, sprint_ms(100000));
        big = std::min(big, sprint_ms(1000000));
    }
    CHECK(big < small * 100 + 100);
    char buf[128];
    std::snprintf(buf, sizeof buf, "Sprintln of 1e5 ints: %.1f ms, 1e6: %.1f ms", small, big);
    return buf;
}

// The values of a runtime panic print with their Error(), and %T gives their Go type.
void test_runtime_errors() {
    gp::any e;
    try {
        gp::panic_index(5, true, 3);
    } catch (const gp::panic_error& p) {
        e = p.gp_value;
    }
    CHECK_EQ(str(fmt::Sprintf("%v|%T"_gs, e, e)),
             "runtime error: index out of range [5] with length 3|runtime.boundsError");
}

// The Scan family (gocpp/fmtscan.hpp, runtime/src/fmtscan.cpp): the operands are pointers with
// their descriptors, the errors are the runtime's io.EOF and io.ErrUnexpectedEOF, and Scan reads
// standard input itself.
void test_scan() {
    auto pi = gp::new_<int64_t>(0);
    auto pj = gp::new_<int64_t>(0);
    auto ps = gp::new_<gp::string>(gp::string());
    auto pf = gp::new_<double>(0.0);
    const gp::type_desc* ti = gp::td_ptr<int64_t>(&gp::td_int);
    const gp::type_desc* ts = gp::td_ptr<gp::string>(&gp::td_string);
    const gp::type_desc* tf = gp::td_ptr<double>(&gp::td_float64);

    auto [n, err] = fmt::Sscan("12 abc 2.5"_gs, fmt::as(ti, pi), fmt::as(ts, ps), fmt::as(tf, pf));
    CHECK(n == 3 && err == nullptr && *pi == 12 && str(*ps) == "abc" && *pf == 2.5);
    std::tie(n, err) = fmt::Sscan(""_gs, fmt::as(ti, pi));
    CHECK(n == 0 && err == fmt::io_eof());
    std::tie(n, err) = fmt::Sscan("1"_gs, fmt::as(ti, pi), fmt::as(ti, pj));
    CHECK(n == 1 && err == fmt::io_eof());
    std::tie(n, err) = fmt::Sscan("x"_gs, fmt::as(ti, pi));
    CHECK(n == 0 && err != nullptr && str(err.Error()) == "expected integer");
    std::tie(n, err) = fmt::Sscanf("7-8"_gs, "%d-%d"_gs, fmt::as(ti, pi), fmt::as(ti, pj));
    CHECK(n == 2 && err == nullptr && *pi == 7 && *pj == 8);
    std::tie(n, err) = fmt::Sscanf("7"_gs, "%d %d"_gs, fmt::as(ti, pi), fmt::as(ti, pj));
    CHECK(n == 1 && err == fmt::io_eof());
    std::tie(n, err) = fmt::Sscanln("1 2 3"_gs, fmt::as(ti, pi), fmt::as(ti, pj));
    CHECK(n == 2 && str(err.Error()) == "expected newline");
    std::tie(n, err) = fmt::Sscan("99999999999999999999"_gs, fmt::as(ti, pi));
    CHECK(n == 0 && str(err.Error()) ==
                        "strconv.ParseInt: parsing \"99999999999999999999\": value out of range");
    std::tie(n, err) = fmt::Sscan("1"_gs, *pi);
    CHECK(n == 0 && str(err.Error()) == "type not a pointer: int");
    CHECK(fmt::io_eof() == fmt::io_eof() && fmt::io_eof() != fmt::io_unexpected_eof());
    CHECK_EQ(str(fmt::io_eof().Error()), "EOF");
    CHECK_EQ(str(fmt::io_unexpected_eof().Error()), "unexpected EOF");
}

// test_stdin: Scan, Scanln and Scanf read descriptor 0 themselves (the OS layer's test seams put
// a pipe, a directory or nothing on it): the newline rules, the end of the input, an interrupted
// read (EINTR, retried), a non-blocking descriptor that is empty (EAGAIN, waited out) and the
// errors.
void test_stdin() {
    namespace tos = gp::os::testing;
    int w = tos::stdin_pipe(false);
    if (w < 0) {
        return; // this system has no seams
    }
    auto pi = gp::new_<int64_t>(0);
    auto pj = gp::new_<int64_t>(0);
    auto ps = gp::new_<gp::string>(gp::string());
    auto pf = gp::new_<double>(0.0);
    const gp::type_desc* ti = gp::td_ptr<int64_t>(&gp::td_int);
    const gp::type_desc* ts = gp::td_ptr<gp::string>(&gp::td_string);
    const gp::type_desc* tf = gp::td_ptr<double>(&gp::td_float64);
    const char text[] = "5 6\nseven\n8.5\n";
    tos::fd_write(w, text, sizeof text - 1);
    tos::fd_close(w);
    int64_t n;
    gp::error err;
    std::tie(n, err) = fmt::Scan(fmt::as(ti, pi), fmt::as(ti, pj));
    CHECK(n == 2 && err == nullptr && *pi == 5 && *pj == 6);
    std::tie(n, err) = fmt::Scanln(fmt::as(ts, ps));
    CHECK(n == 1 && err == nullptr && str(*ps) == "seven");
    std::tie(n, err) = fmt::Scanf("%f"_gs, fmt::as(tf, pf));
    CHECK(n == 1 && err == nullptr && *pf == 8.5);
    std::tie(n, err) = fmt::Scan(fmt::as(ti, pi));
    CHECK(n == 0 && err == fmt::io_eof());
    tos::restore_stdin();

    // An empty non-blocking pipe: the read waits for the writer (EAGAIN), and an alarm interrupts
    // it on the way (EINTR): both are retried.
    w = tos::stdin_pipe(true);
    CHECK(w >= 0);
    std::thread writer([w] {
        std::this_thread::sleep_for(std::chrono::milliseconds(120));
        gp::os::testing::fd_write(w, "42\n", 3);
        gp::os::testing::fd_close(w);
    });
    CHECK(tos::interrupt_in(30));
    std::tie(n, err) = fmt::Scan(fmt::as(ti, pi));
    CHECK(n == 1 && err == nullptr && *pi == 42);
    writer.join();
    tos::restore_stdin();

    // A blocking pipe that is empty: the alarm interrupts the read itself (EINTR), which is
    // retried.
    w = tos::stdin_pipe(false);
    CHECK(w >= 0);
    std::thread writer2([w] {
        std::this_thread::sleep_for(std::chrono::milliseconds(150));
        gp::os::testing::fd_write(w, "43\n", 3);
        gp::os::testing::fd_close(w);
    });
    CHECK(tos::interrupt_in(30));
    std::tie(n, err) = fmt::Scan(fmt::as(ti, pi));
    CHECK(n == 1 && err == nullptr && *pi == 43);
    writer2.join();
    tos::restore_stdin();

    // A directory and a closed descriptor: the read errors, as Go's os.Stdin reports them.
    CHECK(tos::stdin_directory());
    CHECK(tos::stdin_closed());
    tos::restore_stdin();
    if (tos::stdin_directory()) {
        std::tie(n, err) = fmt::Scan(fmt::as(ti, pi));
        CHECK(n == 0 && err != nullptr && str(err.Error()) == "read /dev/stdin: is a directory");
        tos::restore_stdin();
    }
    if (tos::stdin_closed()) {
        std::tie(n, err) = fmt::Scanln(fmt::as(ti, pi));
        CHECK(n == 0 && err != nullptr &&
              str(err.Error()) == "read /dev/stdin: bad file descriptor");
        tos::restore_stdin();
    }
}

// test_gate_start: with a process-wide barrier (membarrier(2), Linux >= 4.14) the gate starts open,
// and a Print on the fast path leaves its thread's slot clear.
void test_gate_start() {
    if (!gp::os::process_barrier_init()) {
        return; // every Print takes the counted slow path on this system
    }
    CHECK((gp::fmt::detail::stdout_gate.word.load() & 1) == 0);
    fmt::Print(gp::string()); // writes nothing
    CHECK(gp::fmt::detail::tls_print_slot != nullptr &&
          gp::fmt::detail::tls_print_slot->inflight.load() == 0);
}

// test_close_defers: defer_close (the close hook of File.Close of the original os.Stdout /
// os.Stdin) closes at once when nothing is in flight, and else keeps the close for the last one out
// (finish_close): a thread plays the Print (a slot announcement) or the Scan (the counter) and
// finishes 100 ms later. The stdout close also makes every thread pass a memory barrier
// (process_barrier).
void test_close_defers() {
    for (bool out : {false, true}) {
        auto& g = out ? gp::fmt::detail::stdout_gate : gp::fmt::detail::stdin_gate;
        int closed = 0;
        // Nothing in flight: the caller closes.
        unsigned long b0 = gp::os::testing::barrier_calls();
        CHECK(!gp::fmt::defer_close(out, [&closed] { closed++; }));
        CHECK(closed == 0);
        CHECK(!out || gp::os::testing::barrier_calls() > b0);
        CHECK((g.word.load() & 1) == 1);
        // In flight: the close waits for the last one.
        std::atomic<int> state{0};
        std::thread t([&] {
            gp::fmt::detail::print_slot* sl = nullptr;
            if (out) {
                sl = gp::fmt::detail::register_print_slot();
                sl->inflight.store(1);
            } else {
                g.word.fetch_add(2);
            }
            state.store(1);
            std::this_thread::sleep_for(std::chrono::milliseconds(100));
            state.store(2);
            if (out) {
                sl->inflight.store(0);
            } else {
                g.word.fetch_sub(2);
            }
            gp::fmt::detail::finish_close(out);
            state.store(3);
        });
        while (state.load() == 0) {
            std::this_thread::yield();
        }
        CHECK(gp::fmt::defer_close(out, [&closed] { closed++; }));
        CHECK(closed == 0); // Close returned without closing the descriptor
        while (state.load() != 3) {
            std::this_thread::yield();
        }
        t.join();
        CHECK(closed == 1); // the last one out closed it, once
        gp::fmt::detail::finish_close(out);
        CHECK(closed == 1);
    }
}

void run_tests() {
    test_gate_start();
    test_close_defers();
    test_scan();
    test_stdin();
    test_descriptors();
    test_operands();
    test_structs();
    test_methods();
    test_panic_and_formatter();
    test_errorf();
    test_changed_slice();
    test_addresses();
    test_pointer_keys_and_weak();
    test_sprint();
    test_runtime_errors();
    std::string speed = test_speed();
    rt_test::threads_ended();
    if (failures > 0) {
        std::string msg =
            "FAIL: " + std::to_string(failures) + " of " + std::to_string(checks) + " checks\n";
        gp::os::write(gp::os::stderr_fd, msg.data(), msg.size());
        gp::os::exit(1);
    }
    fmt::Printf("ok: %d checks (%s)\n"_gs, int64_t(checks), gp::string::gp_copy(speed));
}

void no_init() {}

} // namespace gpkg_main

int main(int argc, char** argv) {
    return gp::run_main(argc, argv, &gpkg_main::no_init, &gpkg_main::run_tests);
}
