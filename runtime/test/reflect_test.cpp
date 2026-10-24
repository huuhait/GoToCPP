// The unit tests of the native functions of package reflect (gocpp/reflect.hpp, Phase 09 step 9.3),
// over types written the way the emitter writes them (a named struct with a complete descriptor,
// gp_full), and over the runtime's own. runtime/cxx_test.go compiles this file with the runtime's
// .cpp files, with g++ and clang++, at -O0, at -O2 and with ASan + UBSan, and runs it like
// test/unit.cpp. It prints "ok: N checks" and exits 0, or prints each failed check and exits 1.
#include <gocpp/rt.hpp>
#include <gocpp/os.hpp>

#include <cstdint>
#include <string>
#include <utility>

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
#define CHECK_STR(got, want)                                                                       \
    do {                                                                                           \
        std::string gp_got(std::string_view((got).gp_view()));                                     \
        report(gp_got == (want), __FILE__, __LINE__,                                               \
               #got " = \"" + gp_got + "\", want \"" + (want) + "\"");                             \
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

namespace rf = gp::reflect;

std::uint64_t T(const gp::type_desc* d) {
    return reinterpret_cast<std::uintptr_t>(d);
}

// --- the types of a program, as the emitter writes them ---
//
//     type Rec struct {
//         A    int
//         S    string `json:"s"`
//         Xs   []int
//         M    map[string]int
//         Next *Rec
//         f    float64
//     }
//     func (r Rec) Name() string       func (r *Rec) Set(int)       func (r Rec) hidden()
//     type Celsius float64             type Sizer interface{ Size() int }

struct Rec {
    int64_t A = 0;
    gp::string S;
    gp::slice<int64_t> Xs;
    gp::map<gp::string, int64_t> M;
    gp::ptr<Rec> Next;
    double f = 0;
};

struct Celsius : gp::newtype<Celsius, double> {
    using gp::newtype<Celsius, double>::newtype;
};

const gp::type_desc* sizer_desc();
struct Sizer : gp::iface {
    using gp::iface::iface;
    static const gp::type_desc* gp_desc() { return sizer_desc(); }
    int64_t Size() const { return gp::icall<int64_t()>(*this, 0); }
};

gp::string thunk_Rec_Name(const gp::iface&) {
    return "rec"_gs;
}
int64_t thunk_Rec_Size(const gp::iface& self) {
    return gp::unbox<Rec>(self).A;
}
void thunk_pRec_Set(const gp::iface&, int64_t) {}
gp::string thunk_pRec_Name(const gp::iface&) {
    return "rec"_gs;
}
int64_t thunk_pRec_Size(const gp::iface& self) {
    return gp::unbox<gp::ptr<Rec>>(self)->A;
}

const gp::method gp_vm_Rec[] = {
    gp::method_of<gp::string()>("Name", nullptr, "func() string", &thunk_Rec_Name),
    gp::method_of<int64_t()>("Size", nullptr, "func() int", &thunk_Rec_Size)};
const gp::method gp_pm_Rec[] = {
    gp::method_of<gp::string()>("Name", nullptr, "func() string", &thunk_pRec_Name),
    gp::method_of<void(int64_t)>("Set", nullptr, "func(int)", &thunk_pRec_Set),
    gp::method_of<int64_t()>("Size", nullptr, "func() int", &thunk_pRec_Size)};
const gp::imethod gp_im_Sizer[] = {gp::imethod("Size", nullptr, "func() int")};

const gp::type_desc* gp_full_Rec();
const gp::type_desc gp_td_Rec =
    gp::named<Rec>("main.Rec", "main", gp::shape_lazy(gp::shape_struct(nullptr, 0), &gp_full_Rec),
                   gp::methods_of(gp_vm_Rec), gp::methods_of(gp_pm_Rec));
const gp::type_desc gp_td_Celsius =
    gp::named<Celsius>("main.Celsius", "main", gp::shape_basic(gp::kind::Float64));
const gp::type_desc gp_td_Sizer =
    gp::named<Sizer>("main.Sizer", "main", gp::shape_interface(gp_im_Sizer));
const gp::type_desc* sizer_desc() {
    return &gp_td_Sizer;
}

const gp::type_desc* td_slice_int() {
    static const gp::type_desc* const td = gp::td_slice<int64_t>(&gp::td_int);
    return td;
}
const gp::type_desc* td_map_str_int() {
    static const gp::type_desc* const td =
        gp::td_map<gp::string, int64_t>(&gp::td_string, &gp::td_int);
    return td;
}
const gp::type_desc* td_pRec() {
    static const gp::type_desc* const td = gp::td_ptr<Rec>(&gp_td_Rec);
    return td;
}
const gp::type_desc* td_array3() {
    static const gp::type_desc* const td = gp::td_array<int64_t, 3>(&gp::td_int);
    return td;
}
const gp::type_desc* td_slice_string() {
    static const gp::type_desc* const td = gp::td_slice<gp::string>(&gp::td_string);
    return td;
}
const gp::type_desc* td_func() {
    static const gp::type_desc* const td =
        gp::td_func<gp::func<gp::string(int64_t, gp::slice<gp::string>)>>(
            {&gp::td_int, td_slice_string()}, {&gp::td_string}, true);
    return td;
}
const gp::type_desc* td_chan_int() {
    static const gp::type_desc* const td =
        gp::td_chan<gp::chan<int64_t>>(gp::chan_dir::recv, &gp::td_int);
    return td;
}

const gp::type_desc* gp_full_Rec() {
    static const gp::field_desc gp_f[] = {
        gp::field_desc("A", nullptr, &gp::td_int, nullptr, false, gp::offset_of(&Rec::A)),
        gp::field_desc("S", nullptr, &gp::td_string, "json:\"s\"", false, gp::offset_of(&Rec::S)),
        gp::field_desc("Xs", nullptr, td_slice_int(), nullptr, false, gp::offset_of(&Rec::Xs)),
        gp::field_desc("M", nullptr, td_map_str_int(), nullptr, false, gp::offset_of(&Rec::M)),
        gp::field_desc("Next", nullptr, td_pRec(), nullptr, false, gp::offset_of(&Rec::Next)),
        gp::field_desc("f", "main", &gp::td_float64, nullptr, false, gp::offset_of(&Rec::f))};
    static const gp::type_desc gp_d = gp::completed(gp_td_Rec, gp::shape_struct(gp_f, 6));
    return &gp_d;
}

gp::any box(const Rec& r) {
    return gp::to_iface<gp::any>(r, &gp_td_Rec);
}

void test_types() {
    CHECK(rf::t_kind(T(&gp::td_int)) == 2);
    CHECK(rf::t_kind(T(&gp::td_unsafe_pointer)) == 26);
    CHECK(rf::t_kind(T(&gp_td_Rec)) == 25);
    CHECK(rf::t_kind(T(td_slice_int())) == 23);
    CHECK(rf::t_kind(T(td_map_str_int())) == 21);
    CHECK(rf::t_kind(T(&gp_td_Sizer)) == 20);
    CHECK(rf::t_kind(T(td_func())) == 19);
    CHECK(rf::t_kind(T(td_chan_int())) == 18);
    CHECK(rf::t_kind(0) == 0);
    CHECK_STR(rf::t_name(T(&gp_td_Rec)), "Rec");
    CHECK_STR(rf::t_pkgpath(T(&gp_td_Rec)), "main");
    CHECK_STR(rf::t_string(T(&gp_td_Rec)), "main.Rec");
    CHECK_STR(rf::t_name(T(&gp::td_int)), "int");
    CHECK_STR(rf::t_pkgpath(T(&gp::td_int)), "");
    CHECK_STR(rf::t_name(T(td_slice_int())), "");
    CHECK_STR(rf::t_string(T(td_slice_int())), "[]int");
    CHECK_STR(rf::t_string(T(td_map_str_int())), "map[string]int");
    CHECK_STR(rf::t_name(T(&gp::td_unsafe_pointer)), "Pointer");
    CHECK_STR(rf::t_pkgpath(T(&gp::td_unsafe_pointer)), "unsafe");
    CHECK_STR(rf::t_string(T(td_func())), "func(int, ...string) string");
    CHECK(rf::t_variadic(T(td_func())) && rf::t_numin(T(td_func())) == 2 &&
          rf::t_numout(T(td_func())) == 1);
    CHECK(rf::t_in(T(td_func()), 0) == T(&gp::td_int) &&
          rf::t_out(T(td_func()), 0) == T(&gp::td_string));
    CHECK(rf::t_chandir(T(td_chan_int())) == 1);
    CHECK(rf::t_elem(T(td_slice_int())) == T(&gp::td_int));
    CHECK(rf::t_key(T(td_map_str_int())) == T(&gp::td_string) &&
          rf::t_elem(T(td_map_str_int())) == T(&gp::td_int));
    CHECK(rf::t_len(T(td_array3())) == 3 && rf::t_elem(T(td_array3())) == T(&gp::td_int));
    CHECK(rf::t_elem(T(&gp::td_int)) == 0);
    CHECK(rf::t_size(T(&gp::td_int8)) == 1 && rf::t_size(T(&gp::td_int32)) == 4 &&
          rf::t_size(T(&gp_td_Rec)) == sizeof(Rec));
    CHECK(rf::t_kind(T(&gp_td_Celsius)) == 14);
    CHECK_STR(rf::t_name(T(&gp_td_Celsius)), "Celsius");
}

void test_fields_methods() {
    const std::uint64_t t = T(&gp_td_Rec);
    CHECK(rf::t_numfield(t) == 6);
    CHECK_STR(rf::f_name(t, 1), "S");
    CHECK_STR(rf::f_tag(t, 1), "json:\"s\"");
    CHECK_STR(rf::f_tag(t, 0), "");
    CHECK(rf::f_type(t, 2) == T(td_slice_int()));
    CHECK(rf::f_offset(t, 0) == 0 && rf::f_offset(t, 1) == offsetof(Rec, S));
    CHECK(!rf::f_embedded(t, 0));
    CHECK_STR(rf::f_pkgpath(t, 5), "main");
    CHECK_STR(rf::f_pkgpath(t, 0), "");
    CHECK(rf::t_numfield(T(&gp::td_int)) == 0);
    CHECK(rf::t_nmethod(t) == 2);
    CHECK_STR(rf::m_name(t, 0), "Name");
    CHECK_STR(rf::m_sig(t, 1), "func() int");
    CHECK(rf::t_nmethod(T(td_pRec())) == 3);
    CHECK_STR(rf::m_name(T(td_pRec()), 1), "Set");
    CHECK(rf::t_nmethod(T(&gp_td_Sizer)) == 1);
    CHECK_STR(rf::m_name(T(&gp_td_Sizer), 0), "Size");
    CHECK(rf::t_nmethod(T(&gp::td_int)) == 0);
}

void test_relations() {
    CHECK(rf::t_implements(T(&gp_td_Rec), T(&gp_td_Sizer)));
    CHECK(rf::t_implements(T(td_pRec()), T(&gp_td_Sizer)));
    CHECK(!rf::t_implements(T(&gp::td_int), T(&gp_td_Sizer)));
    CHECK(rf::t_implements(T(&gp::td_int), T(&gp::td_any)));
    CHECK(rf::t_implements(T(&gp_td_Sizer), T(&gp_td_Sizer)));
    CHECK(!rf::t_implements(T(&gp::td_any), T(&gp_td_Sizer)));
    CHECK(rf::t_assignable(T(&gp::td_int), T(&gp::td_int)));
    CHECK(!rf::t_assignable(T(&gp::td_int), T(&gp::td_int64)));
    CHECK(!rf::t_assignable(T(&gp_td_Celsius), T(&gp::td_float64)));
    CHECK(rf::t_assignable(T(&gp_td_Rec), T(&gp_td_Sizer)));
    CHECK(rf::t_assignable(T(&gp::td_string), T(&gp::td_any)));
    CHECK(rf::t_convertible(T(&gp_td_Celsius), T(&gp::td_float64)));
    CHECK(rf::t_convertible(T(&gp::td_int), T(&gp::td_float32)));
    CHECK(rf::t_convertible(T(&gp::td_int), T(&gp::td_string)));
    CHECK(!rf::t_convertible(T(&gp::td_string), T(&gp::td_int)));
    CHECK(rf::t_comparable(T(&gp::td_int)));
    CHECK(!rf::t_comparable(T(td_slice_int())));
    CHECK(!rf::t_comparable(T(&gp_td_Rec)));
    CHECK(rf::t_comparable(T(td_array3())));
    CHECK(rf::t_comparable(T(&gp_td_Sizer)));
}

void test_dynamic_types() {
    // The program's own descriptors come back for the types it has.
    CHECK(rf::t_ptrto(T(&gp_td_Rec)) == T(td_pRec()));
    CHECK(rf::t_sliceof(T(&gp::td_int)) == T(td_slice_int()));
    // A type the program never wrote: *string, []float64 are made, once.
    std::uint64_t ps = rf::t_ptrto(T(&gp::td_string));
    CHECK(ps != 0 && rf::t_ptrto(T(&gp::td_string)) == ps);
    CHECK_STR(rf::t_string(ps), "*string");
    CHECK(rf::t_kind(ps) == 22 && rf::t_elem(ps) == T(&gp::td_string));
    std::uint64_t sf = rf::t_sliceof(T(&gp::td_float64));
    CHECK_STR(rf::t_string(sf), "[]float64");
    CHECK(rf::t_sliceof(T(&gp::td_float64)) == sf);
    // The method set of *Rec is carried by the made pointer type too.
    CHECK(rf::t_nmethod(rf::t_ptrto(T(&gp_td_Rec))) == 3);
}

void test_rtype() {
    gp::ptr<void> a = rf::rtype_of(T(&gp::td_int));
    gp::ptr<void> b = rf::rtype_of(T(&gp::td_int));
    gp::ptr<void> c = rf::rtype_of(T(&gp::td_string));
    CHECK(a.gp_get() == b.gp_get() && a.gp_get() != c.gp_get());
    CHECK(*static_cast<std::uint64_t*>(a.gp_get()) == T(&gp::td_int));
}

void test_scalars() {
    gp::any x = gp::to_iface<gp::any>(int64_t(-7), &gp::td_int);
    auto [t, p] = rf::v_unpack(x);
    CHECK(t == T(&gp::td_int) && rf::v_int(t, p) == -7);
    rf::v_setint(t, p, 99);
    CHECK(rf::v_int(t, p) == 99);
    gp::any y = rf::v_interface(t, p);
    CHECK(gp::unbox<int64_t>(y) == 99 && gp::unbox<int64_t>(x) == 99); // the Value shares the box
    // Sized integers wrap as Go's.
    // (A one-byte value, and a small one, is in one of Go's shared boxes, which are never
    // written: the writes go to storage of their own.)
    gp::any i8 = gp::to_iface<gp::any>(int8_t(1), &gp::td_int8);
    auto [t8, b8] = rf::v_unpack(i8);
    gp::ptr<void> p8 = rf::v_new(t8);
    rf::v_assign(t8, p8, b8);
    CHECK(rf::v_int(t8, p8) == 1);
    rf::v_setint(t8, p8, 200);
    CHECK(rf::v_int(t8, p8) == -56);
    gp::any u16 = gp::to_iface<gp::any>(uint16_t(1), &gp::td_uint16);
    auto [tu, bu] = rf::v_unpack(u16);
    gp::ptr<void> pu = rf::v_new(tu);
    rf::v_assign(tu, pu, bu);
    rf::v_setuint(tu, pu, 70000);
    CHECK(rf::v_uint(tu, pu) == 70000 - 65536);
    gp::any f32 = gp::to_iface<gp::any>(1.5f, &gp::td_float32);
    auto [tf, pf] = rf::v_unpack(f32);
    CHECK(rf::v_float(tf, pf) == 1.5);
    rf::v_setfloat(tf, pf, 2.25);
    CHECK(rf::v_float(tf, pf) == 2.25);
    gp::any c64 = gp::to_iface<gp::any>(gp::complex64(1.f, 2.f), &gp::td_complex64);
    auto [tc, pc] = rf::v_unpack(c64);
    CHECK(rf::v_complex(tc, pc).gp_im == 2.0);
    rf::v_setcomplex(tc, pc, gp::complex128(3.0, 4.0));
    CHECK(rf::v_complex(tc, pc).gp_re == 3.0);
    gp::any s = gp::to_iface<gp::any>(gp::string::gp_copy("hi"), &gp::td_string);
    auto [ts, ps] = rf::v_unpack(s);
    CHECK_STR(rf::v_string(ts, ps), "hi");
    rf::v_setstring(ts, ps, gp::string::gp_copy("changed"));
    CHECK_STR(rf::v_string(ts, ps), "changed");
    CHECK(rf::v_len(ts, ps) == 7);
    gp::any bl = gp::to_iface<gp::any>(true, &gp::td_bool);
    auto [tb, bb] = rf::v_unpack(bl);
    CHECK(rf::v_bool(tb, bb));
    gp::ptr<void> pb = rf::v_new(tb);
    rf::v_assign(tb, pb, bb);
    rf::v_setbool(tb, pb, false);
    CHECK(!rf::v_bool(tb, pb));
    // Celsius is a named float: read through its own descriptor.
    gp::any ce = gp::to_iface<gp::any>(Celsius(36.5), &gp_td_Celsius);
    auto [tn, pn] = rf::v_unpack(ce);
    CHECK(rf::v_float(tn, pn) == 36.5);
    // The nil interface
    auto [t0, p0] = rf::v_unpack(gp::any());
    CHECK(t0 == 0 && p0 == nullptr);
}

// Go's shared boxes (runtime.staticuint64s, zeroVal, zerobase): two interfaces of such a value give
// the same storage, any other value its own box.
void test_shared_boxes() {
    auto same = [](const gp::any& a, const gp::any& b) {
        return std::get<1>(rf::v_unpack(a)).gp_get() == std::get<1>(rf::v_unpack(b)).gp_get();
    };
    auto i = [](int64_t v) { return gp::to_iface<gp::any>(v, &gp::td_int); };
    CHECK(same(i(1), i(1)) && same(i(0), i(0)) && same(i(255), i(255)));
    CHECK(!same(i(256), i(256)) && !same(i(-1), i(-1)) && !same(i(1), i(2)));
    CHECK(rf::v_int(T(&gp::td_int), std::get<1>(rf::v_unpack(i(7)))) == 7);
    auto i8 = [](int8_t v) { return gp::to_iface<gp::any>(v, &gp::td_int8); };
    CHECK(same(i8(-1), i8(-1)) &&
          rf::v_int(T(&gp::td_int8), std::get<1>(rf::v_unpack(i8(-3)))) == -3);
    auto u16 = [](uint16_t v) { return gp::to_iface<gp::any>(v, &gp::td_uint16); };
    CHECK(same(u16(200), u16(200)) && !same(u16(300), u16(300)));
    CHECK(rf::v_uint(T(&gp::td_uint16), std::get<1>(rf::v_unpack(u16(200)))) == 200);
    auto f64 = [](double v) { return gp::to_iface<gp::any>(v, &gp::td_float64); };
    CHECK(same(f64(0.0), f64(0.0)) && !same(f64(1.0), f64(1.0)) && !same(f64(-0.0), f64(-0.0)));
    auto b = [](bool v) { return gp::to_iface<gp::any>(v, &gp::td_bool); };
    CHECK(same(b(true), b(true)) && same(b(false), b(false)) && !same(b(true), b(false)));
    auto c64 = [](float r) {
        return gp::to_iface<gp::any>(gp::complex64(r, 0.f), &gp::td_complex64);
    };
    CHECK(!same(c64(0.f), c64(0.f))); // 4-aligned: Go's convTnoptr allocates
    auto s = [](const char* v) {
        return gp::to_iface<gp::any>(gp::string::gp_copy(v), &gp::td_string);
    };
    CHECK(same(s(""), s("")) && !same(s("a"), s("a")));
    CHECK(rf::v_len(T(&gp::td_string), std::get<1>(rf::v_unpack(s("")))) == 0);
}

void test_struct_and_pointer() {
    Rec r;
    r.A = 5;
    r.S = gp::string::gp_copy("s");
    gp::ptr<Rec> pr = gp::new_<Rec>(r);
    gp::any x = gp::to_iface<gp::any>(pr, td_pRec());
    auto [t, p] = rf::v_unpack(x);
    CHECK(t == T(td_pRec()));
    CHECK(!rf::v_isnil(t, p));
    gp::ptr<void> e = rf::v_elem(t, p);
    CHECK(e.gp_get() == pr.gp_get());
    gp::ptr<void> fa = rf::v_offset(e, rf::f_offset(T(&gp_td_Rec), 0));
    rf::v_setint(T(&gp::td_int), fa, 77);
    CHECK(pr->A == 77);
    gp::ptr<void> fs = rf::v_offset(e, rf::f_offset(T(&gp_td_Rec), 1));
    CHECK_STR(rf::v_string(T(&gp::td_string), fs), "s");
    // Addr: a pointer to a field keeps the struct alive.
    gp::ptr<void> pa =
        rf::v_ptrto(T(rf::t_ptrto(T(&gp::td_int))
                          ? reinterpret_cast<const gp::type_desc*>(rf::t_ptrto(T(&gp::td_int)))
                          : nullptr),
                    fa);
    gp::ptr<void> back = rf::v_elem(rf::t_ptrto(T(&gp::td_int)), pa);
    CHECK(back.gp_get() == fa.gp_get());
    pr = nullptr; // the block lives on through fa
    rf::v_setint(T(&gp::td_int), back, 78);
    CHECK(rf::v_int(T(&gp::td_int), fa) == 78);
    // A nil pointer
    gp::any nilp = gp::to_iface<gp::any>(gp::ptr<Rec>(), td_pRec());
    auto [tn, pn] = rf::v_unpack(nilp);
    CHECK(rf::v_isnil(tn, pn));
    CHECK(rf::v_elem(tn, pn) == nullptr);
    gp::any back_to_iface = rf::v_interface(tn, pn);
    CHECK(back_to_iface.gp_type() == td_pRec() && back_to_iface.gp_data() == nullptr);
}

void test_new_assign() {
    gp::ptr<void> n = rf::v_new(T(&gp_td_Rec));
    auto* rec = static_cast<Rec*>(n.gp_get());
    CHECK(rec->A == 0 && rec->S.gp_view().empty() && rec->Xs == nullptr);
    rec->A = 3;
    rec->S = gp::string::gp_copy("assigned");
    gp::ptr<void> m = rf::v_new(T(&gp_td_Rec));
    rf::v_assign(T(&gp_td_Rec), m, n);
    CHECK(static_cast<Rec*>(m.gp_get())->A == 3);
    CHECK(std::string(static_cast<Rec*>(m.gp_get())->S.gp_view()) == "assigned");
    // Interface storage: a zero any, then a value assigned with its itab
    gp::ptr<void> iv = rf::v_new(T(&gp_td_Sizer));
    CHECK(static_cast<gp::iface*>(iv.gp_get())->gp_type() == nullptr);
    gp::any boxed = box(*rec);
    auto [tt, pp] = rf::v_unpack(boxed);
    rf::v_assign_iface(T(&gp_td_Sizer), iv, tt, pp);
    auto* si = static_cast<Sizer*>(iv.gp_get());
    CHECK(si->gp_type() == &gp_td_Rec && si->gp_tab() != nullptr && si->Size() == 3);
    gp::any out = rf::v_interface(T(&gp_td_Sizer), iv);
    CHECK(out.gp_type() == &gp_td_Rec && out.gp_tab() == nullptr);
    // A type that can't be made
    CHECK(panic_text([] { rf::v_new(T(&gp::td_unsafe_pointer)); }) == "<no panic>");
}

void test_slices() {
    const std::uint64_t st = T(td_slice_int());
    gp::ptr<void> s = rf::v_mkslice(st, 3, 5);
    CHECK(rf::v_len(st, s) == 3 && rf::v_cap(st, s) == 5 && !rf::v_isnil(st, s));
    for (int i = 0; i < 3; i++) {
        rf::v_setint(T(&gp::td_int), rf::v_index(st, s, i), i * 10);
    }
    auto& real = *static_cast<gp::slice<int64_t>*>(s.gp_get());
    CHECK(real[0] == 0 && real[1] == 10 && real[2] == 20);
    gp::ptr<void> sub = rf::v_reslice(st, s, st, 1, 3, 5);
    CHECK(rf::v_len(st, sub) == 2 && rf::v_cap(st, sub) == 4);
    CHECK(rf::v_int(T(&gp::td_int), rf::v_index(st, sub, 0)) == 10);
    rf::v_setlen(st, s, 5);
    CHECK(real.gp_len() == 5 && real[4] == 0);
    // extend within capacity shares the array; beyond it makes a new one with the old elements
    rf::v_setlen(st, s, 3);
    gp::ptr<void> e1 = rf::v_extend(st, s, 2);
    CHECK(rf::v_len(st, e1) == 5 &&
          static_cast<gp::slice<int64_t>*>(e1.gp_get())->gp_data() == real.gp_data());
    gp::ptr<void> e2 = rf::v_extend(st, s, 10);
    auto& big = *static_cast<gp::slice<int64_t>*>(e2.gp_get());
    CHECK(big.gp_len() == 13 && big.gp_cap() >= 13 && big.gp_data() != real.gp_data() &&
          big[2] == 20 && big[12] == 0);
    // copy
    gp::ptr<void> d = rf::v_mkslice(st, 2, 2);
    CHECK(rf::v_copy(st, d, st, s) == 2);
    CHECK(rf::v_int(T(&gp::td_int), rf::v_index(st, d, 1)) == 10);
    // a slice of strings: the elements are made with their own lifetime
    const std::uint64_t sst = T(td_slice_string());
    gp::ptr<void> ss = rf::v_mkslice(sst, 2, 4);
    rf::v_setstring(T(&gp::td_string), rf::v_index(sst, ss, 1), gp::string::gp_copy("x"));
    gp::ptr<void> ss2 = rf::v_extend(sst, ss, 5);
    CHECK_STR(rf::v_string(T(&gp::td_string), rf::v_index(sst, ss2, 1)), "x");
    // an array's elements and the slice of an array
    gp::ptr<void> arr = rf::v_new(T(td_array3()));
    rf::v_setint(T(&gp::td_int), rf::v_index(T(td_array3()), arr, 2), 9);
    gp::ptr<void> as = rf::v_reslice(T(td_array3()), arr, st, 1, 3, 3);
    CHECK(rf::v_len(st, as) == 2 && rf::v_int(T(&gp::td_int), rf::v_index(st, as, 1)) == 9);
    CHECK(rf::v_len(T(td_array3()), arr) == 3 && rf::v_cap(T(td_array3()), arr) == 3);
    // a slice of a type the program never wrote
    const std::uint64_t sf = rf::t_sliceof(T(&gp::td_float64));
    gp::ptr<void> fs = rf::v_mkslice(sf, 2, 2);
    rf::v_setfloat(T(&gp::td_float64), rf::v_index(sf, fs, 1), 2.5);
    CHECK(static_cast<gp::slice<double>*>(fs.gp_get())->operator[](1) == 2.5);
    // a string is sliced too
    gp::any str = gp::to_iface<gp::any>(gp::string::gp_copy("hello"), &gp::td_string);
    auto [tt, pp] = rf::v_unpack(str);
    gp::ptr<void> sl = rf::v_reslice(tt, pp, tt, 1, 3, 3);
    CHECK_STR(rf::v_string(tt, sl), "el");
}

void test_maps() {
    const std::uint64_t mt = T(td_map_str_int());
    gp::ptr<void> m = rf::v_mapnew(mt);
    CHECK(!rf::v_isnil(mt, m) && rf::v_len(mt, m) == 0);
    gp::ptr<void> k = rf::v_new(T(&gp::td_string));
    gp::ptr<void> v = rf::v_new(T(&gp::td_int));
    rf::v_setstring(T(&gp::td_string), k, gp::string::gp_copy("a"));
    rf::v_setint(T(&gp::td_int), v, 1);
    rf::v_mapset(mt, m, k, v);
    rf::v_setstring(T(&gp::td_string), k, gp::string::gp_copy("b"));
    rf::v_setint(T(&gp::td_int), v, 2);
    rf::v_mapset(mt, m, k, v);
    CHECK(rf::v_len(mt, m) == 2);
    gp::ptr<void> got = rf::v_mapindex(mt, m, k);
    CHECK(got != nullptr && rf::v_int(T(&gp::td_int), got) == 2);
    rf::v_setstring(T(&gp::td_string), k, gp::string::gp_copy("zz"));
    CHECK(rf::v_mapindex(mt, m, k) == nullptr);
    CHECK(!rf::v_maphas(mt, m, k));
    rf::v_setstring(T(&gp::td_string), k, gp::string::gp_copy("b"));
    CHECK(rf::v_maphas(mt, m, k));
    rf::v_setstring(T(&gp::td_string), k, gp::string::gp_copy("zz"));
    gp::ptr<void> snap = rf::v_mapsnap(mt, m);
    CHECK(rf::v_snaplen(mt, snap) == 2);
    int64_t sum = 0;
    for (int i = 0; i < 2; i++) {
        sum += rf::v_int(T(&gp::td_int), rf::v_snapval(mt, snap, i));
        CHECK(rf::v_string(T(&gp::td_string), rf::v_snapkey(mt, snap, i)).gp_view().size() == 1);
    }
    CHECK(sum == 3);
    rf::v_setstring(T(&gp::td_string), k, gp::string::gp_copy("a"));
    rf::v_mapdel(mt, m, k);
    CHECK(rf::v_len(mt, m) == 1);
    rf::v_mapclear(mt, m);
    CHECK(rf::v_len(mt, m) == 0);
    // a nil map: reads are empty, a store panics with Go's text
    gp::ptr<void> nm = rf::v_new(mt);
    CHECK(rf::v_isnil(mt, nm) && rf::v_mapindex(mt, nm, k) == nullptr);
    CHECK(panic_text([&] { rf::v_mapset(mt, nm, k, v); }) == "assignment to entry in nil map");
}

void test_equal_convert() {
    gp::ptr<void> a = rf::v_new(T(&gp::td_int));
    gp::ptr<void> b = rf::v_new(T(&gp::td_int));
    rf::v_setint(T(&gp::td_int), a, 4);
    rf::v_setint(T(&gp::td_int), b, 4);
    CHECK(rf::v_equal(T(&gp::td_int), a, b));
    rf::v_setint(T(&gp::td_int), b, 5);
    CHECK(!rf::v_equal(T(&gp::td_int), a, b));
    CHECK(panic_text([&] { rf::v_equal(T(td_slice_int()), a, b); }).find("uncomparable") !=
          std::string::npos);
    // conversions
    gp::ptr<void> f = rf::v_convert(T(&gp::td_int), a, T(&gp::td_float64));
    CHECK(rf::v_float(T(&gp::td_float64), f) == 4.0);
    rf::v_setfloat(T(&gp::td_float64), f, -3.99);
    gp::ptr<void> i = rf::v_convert(T(&gp::td_float64), f, T(&gp::td_int));
    CHECK(rf::v_int(T(&gp::td_int), i) == -3);
    rf::v_setint(T(&gp::td_int), a, 65);
    gp::ptr<void> s = rf::v_convert(T(&gp::td_int), a, T(&gp::td_string));
    CHECK_STR(rf::v_string(T(&gp::td_string), s), "A");
    gp::ptr<void> i8 = rf::v_convert(T(&gp::td_int), a, T(&gp::td_int8));
    CHECK(rf::v_int(T(&gp::td_int8), i8) == 65);
    rf::v_setint(T(&gp::td_int), a, 300);
    gp::ptr<void> w = rf::v_convert(T(&gp::td_int), a, T(&gp::td_uint8));
    CHECK(rf::v_uint(T(&gp::td_uint8), w) == 44);
}

void test_swapper() {
    gp::slice<int64_t> xs = gp::slice_of<int64_t>({1, 2, 3});
    auto sw = rf::swapper(gp::to_iface<gp::any>(xs, td_slice_int()));
    sw(0, 2);
    CHECK(xs[0] == 3 && xs[2] == 1);
    CHECK(panic_text([&] { sw(0, 3); }) == "runtime error: index out of range [3] with length 3");
    CHECK(panic_text([&] { sw(-1, 0); }) == "runtime error: index out of range [-1]");
    gp::slice<Rec> recs = gp::slice_of<Rec>({Rec{}, Rec{}});
    recs[0].A = 1;
    recs[1].A = 2;
    const gp::type_desc* td_slice_rec = gp::td_slice<Rec>(&gp_td_Rec);
    auto sw2 = rf::swapper(gp::to_iface<gp::any>(recs, td_slice_rec));
    sw2(0, 1);
    CHECK(recs[0].A == 2 && recs[1].A == 1);
    CHECK(panic_text([&] { sw2(0, 2); }) == "reflect: slice index out of range");
    gp::slice<gp::string> strs =
        gp::slice_of<gp::string>({gp::string::gp_copy("a"), gp::string::gp_copy("b")});
    rf::swapper(gp::to_iface<gp::any>(strs, td_slice_string()))(0, 1);
    CHECK(strs[0].gp_view() == "b" && strs[1].gp_view() == "a");
    gp::slice<int64_t> empty = gp::slice_of<int64_t>({});
    auto sw0 = rf::swapper(gp::to_iface<gp::any>(empty, td_slice_int()));
    CHECK(panic_text([&] { sw0(0, 0); }) == "reflect: slice index out of range");
}

void test_foreign() {
    gp::any n = gp::to_iface_foreign<gp::any>(int32_t(7));
    auto [t, p] = rf::v_unpack(n);
    CHECK(rf::t_kind(t) == 5 && rf::v_int(t, p) == 7);
    CHECK_STR(rf::t_string(t), "int32");
    CHECK_STR(rf::t_cppname(t), "int");
    CHECK_STR(rf::t_cppname(T(&gp::td_int)), "");
    gp::any d = gp::to_iface_foreign<gp::any>(2.5);
    auto [td, pd] = rf::v_unpack(d);
    CHECK(rf::t_kind(td) == 14 && rf::v_float(td, pd) == 2.5);
    gp::any s = gp::to_iface_foreign<gp::any>(std::string("cxx"));
    auto [ts, ps] = rf::v_unpack(s);
    CHECK(rf::t_kind(ts) == 24);
    CHECK_STR(rf::v_string(ts, ps), "cxx");
    CHECK(rf::v_len(ts, ps) == 3);
    int x = 0;
    gp::any ptr = gp::to_iface_foreign<gp::any>(&x);
    auto [tp, pp] = rf::v_unpack(ptr);
    CHECK(rf::t_kind(tp) == 26 && !rf::v_isnil(tp, pp));
    CHECK(rf::v_unsafeptr(tp, pp).gp_get() == &x);
    // a class: a struct without fields
    gp::any cls = gp::to_iface_foreign<gp::any>(std::pair<int, int>(1, 2));
    auto [tc, pc] = rf::v_unpack(cls);
    CHECK(rf::t_kind(tc) == 25 && rf::t_numfield(tc) == 0);
    gp::any back = rf::v_interface(tc, pc);
    CHECK(T(back.gp_type()) == tc);
}

} // namespace

int main() {
    test_types();
    test_fields_methods();
    test_relations();
    test_dynamic_types();
    test_rtype();
    test_scalars();
    test_shared_boxes();
    test_struct_and_pointer();
    test_new_assign();
    test_slices();
    test_maps();
    test_equal_convert();
    test_swapper();
    test_foreign();
    if (failures != 0) {
        return 1;
    }
    std::string msg = "ok: " + std::to_string(checks) + " checks\n";
    gp::os::write(gp::os::stdout_fd, msg.data(), msg.size());
    return 0;
}
