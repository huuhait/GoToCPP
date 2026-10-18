// The unit tests of the conversions between interface types, the type assertions and the helpers
// of type switches (gocpp/ifaceconv.hpp, Phase 05 steps 5.4 and 5.5), and of gp::error
// (gocpp/error.hpp). The types are written the way the emitter writes them. runtime/cxx_test.go
// compiles this file with the runtime's .cpp files, with g++ and clang++, at -O0, at -O2 and with
// ASan + UBSan, and runs it like test/unit.cpp. It prints "ok: N checks" and exits 0, or prints
// each failed check and exits 1.
#include <gocpp/rt.hpp>
#include <gocpp/os.hpp>

#include <cstdint>
#include <cstdlib>
#include <new>
#include <set>
#include <string>
#include <tuple>
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

// The number of allocations (operator new), to show that an assertion allocates nothing.
std::size_t allocations = 0;

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

// --- the types of a program, as the emitter writes them ---
//
//     type Shape interface{ Area() float64; Scale(k float64) }
//     type Areaer interface{ Area() float64 }
//     type Namer interface{ Name() string }
//     type Empty interface{}
//     type Rect struct{ W, H float64 }          Area, Name: value methods; Scale: pointer method
//     type Circle struct{ R float64 }           (c *Circle) Area()
//     type Celsius float64                      no methods
//     type MyErr struct{ msg string }           (e *MyErr) Error() string

struct Rect {
    double W = 0;
    double H = 0;
    bool operator==(const Rect&) const = default;
    std::size_t gp_hash() const { return gp::hash_fields(W, H); }
    double Area() const { return W * H; }
    gp::string Name() const { return "rect"_gs; }
    static void Scale(gp::ptr<Rect> r, double k) {
        r->W *= k;
        r->H *= k;
    }
};

struct Circle {
    double R = 0;
    static double Area(gp::ptr<Circle> c) { return 3 * c->R * c->R; }
};

struct MyErr {
    gp::string msg;
    static gp::string Error(gp::ptr<MyErr> e) { return e->msg; }
};

struct Celsius : gp::newtype<Celsius, double> {
    using gp::newtype<Celsius, double>::newtype;
};

const gp::type_desc* shape_desc();
const gp::type_desc* areaer_desc();
const gp::type_desc* namer_desc();
const gp::type_desc* empty_desc();

struct Shape : gp::iface {
    using gp::iface::iface;
    static const gp::type_desc* gp_desc() { return shape_desc(); }
    double Area() const;
    void Scale(double k) const;
};
struct Areaer : gp::iface {
    using gp::iface::iface;
    static const gp::type_desc* gp_desc() { return areaer_desc(); }
    double Area() const;
};
struct Namer : gp::iface {
    using gp::iface::iface;
    static const gp::type_desc* gp_desc() { return namer_desc(); }
    gp::string Name() const;
};
struct Empty : gp::iface {
    using gp::iface::iface;
    static const gp::type_desc* gp_desc() { return empty_desc(); }
};

inline double Shape::Area() const {
    return gp::icall<double()>(*this, 0);
}
inline void Shape::Scale(double k) const {
    gp::icall<void(double)>(*this, 1, k);
}
inline double Areaer::Area() const {
    return gp::icall<double()>(*this, 0);
}
inline gp::string Namer::Name() const {
    return gp::icall<gp::string()>(*this, 0);
}

double thunk_Rect_Area(const gp::iface& self) {
    return gp::unbox<Rect>(self).Area();
}
gp::string thunk_Rect_Name(const gp::iface& self) {
    return gp::unbox<Rect>(self).Name();
}
double thunk_pRect_Area(const gp::iface& self) {
    gp::ptr<Rect> p = gp::unbox<gp::ptr<Rect>>(self);
    gp::check_wrap(p, "value method main.Rect.Area called using nil *Rect pointer");
    return p->Area();
}
gp::string thunk_pRect_Name(const gp::iface& self) {
    gp::ptr<Rect> p = gp::unbox<gp::ptr<Rect>>(self);
    gp::check_wrap(p, "value method main.Rect.Name called using nil *Rect pointer");
    return p->Name();
}
void thunk_pRect_Scale(const gp::iface& self, double k) {
    Rect::Scale(gp::unbox<gp::ptr<Rect>>(self), k);
}
double thunk_pCircle_Area(const gp::iface& self) {
    return Circle::Area(gp::unbox<gp::ptr<Circle>>(self));
}
gp::string thunk_pMyErr_Error(const gp::iface& self) {
    return MyErr::Error(gp::unbox<gp::ptr<MyErr>>(self));
}

const gp::method gp_vm_Rect[] = {
    gp::method_of<double()>("Area", nullptr, "func() float64", &thunk_Rect_Area),
    gp::method_of<gp::string()>("Name", nullptr, "func() string", &thunk_Rect_Name),
};
const gp::method gp_pm_Rect[] = {
    gp::method_of<double()>("Area", nullptr, "func() float64", &thunk_pRect_Area),
    gp::method_of<gp::string()>("Name", nullptr, "func() string", &thunk_pRect_Name),
    gp::method_of<void(double)>("Scale", nullptr, "func(float64)", &thunk_pRect_Scale),
};
const gp::method gp_pm_Circle[] = {
    gp::method_of<double()>("Area", nullptr, "func() float64", &thunk_pCircle_Area),
};
const gp::method gp_pm_MyErr[] = {
    gp::method_of<gp::string()>("Error", nullptr, "func() string", &thunk_pMyErr_Error),
};

const gp::imethod gp_im_Shape[] = {gp::imethod("Area", nullptr, "func() float64"),
                                   gp::imethod("Scale", nullptr, "func(float64)")};
const gp::imethod gp_im_Areaer[] = {gp::imethod("Area", nullptr, "func() float64")};
const gp::imethod gp_im_Namer[] = {gp::imethod("Name", nullptr, "func() string")};

const gp::type_desc gp_td_Rect =
    gp::named<Rect>("main.Rect", "main", gp::shape_struct(nullptr, 0), gp::methods_of(gp_vm_Rect),
                    gp::methods_of(gp_pm_Rect));
const gp::type_desc gp_td_Circle = gp::named<Circle>(
    "main.Circle", "main", gp::shape_struct(nullptr, 0), {}, gp::methods_of(gp_pm_Circle));
const gp::type_desc gp_td_MyErr = gp::named<MyErr>(
    "main.MyErr", "main", gp::shape_struct(nullptr, 0), {}, gp::methods_of(gp_pm_MyErr));
const gp::type_desc gp_td_Celsius =
    gp::named<Celsius>("main.Celsius", "main", gp::shape_basic(gp::kind::Float64));
const gp::type_desc gp_td_Shape =
    gp::named<Shape>("main.Shape", "main", gp::shape_interface(gp_im_Shape));
const gp::type_desc gp_td_Areaer =
    gp::named<Areaer>("main.Areaer", "main", gp::shape_interface(gp_im_Areaer));
const gp::type_desc gp_td_Namer =
    gp::named<Namer>("main.Namer", "main", gp::shape_interface(gp_im_Namer));
const gp::type_desc gp_td_Empty =
    gp::named<Empty>("main.Empty", "main", gp::shape_interface(nullptr, 0));

const gp::type_desc* shape_desc() {
    return &gp_td_Shape;
}
const gp::type_desc* areaer_desc() {
    return &gp_td_Areaer;
}
const gp::type_desc* namer_desc() {
    return &gp_td_Namer;
}
const gp::type_desc* empty_desc() {
    return &gp_td_Empty;
}

const gp::type_desc* td_pRect() {
    static const gp::type_desc* const td = gp::td_ptr<Rect>(&gp_td_Rect);
    return td;
}
[[maybe_unused]] const gp::type_desc* td_pCircle() {
    static const gp::type_desc* const td = gp::td_ptr<Circle>(&gp_td_Circle);
    return td;
}
const gp::type_desc* td_pMyErr() {
    static const gp::type_desc* const td = gp::td_ptr<MyErr>(&gp_td_MyErr);
    return td;
}

// A second Rect type with the same Go name, in another scope/package.
struct Rect2 {
    double W = 0;
};
const gp::type_desc gp_td_Rect2_pkg =
    gp::named<Rect2>("main.Rect", "other/pkg", gp::shape_struct(nullptr, 0));
const gp::type_desc gp_td_Rect2_scope =
    gp::named<Rect2>("main.Rect", "main", gp::shape_struct(nullptr, 0));

Shape make_shape_rect_ptr(double w, double h) {
    return gp::to_iface<Shape>(gp::new_<Rect>(Rect{w, h}), td_pRect());
}

// --- conversions between interface types ---

void test_iface_cast() {
    using namespace gp;
    Shape s = make_shape_rect_ptr(2, 3);
    // Widening to a smaller method set.
    Areaer a = iface_cast<Areaer>(s);
    CHECK(a != nullptr && a.gp_type() == td_pRect() && a.Area() == 6);
    CHECK(a.gp_data() == s.gp_data()); // the box is shared
    CHECK(a.gp_tab() != nullptr && a.gp_tab() != s.gp_tab() &&
          a.gp_tab()->gp_inter == &gp_td_Areaer);
    // To any: no itab.
    any e = iface_cast<any>(s);
    CHECK(e.gp_type() == td_pRect() && e.gp_tab() == nullptr);
    Empty em = iface_cast<Empty>(s);
    CHECK(em.gp_type() == td_pRect() && em.gp_tab() == nullptr);
    // Back to a named interface with more methods than the source, through any: a conversion
    // with a check (the static widening does not need it).
    Namer n = iface_cast<Namer>(e); // *Rect has Name
    CHECK(n.Name() == "rect"_gs);
    // nil in, nil out, for every target.
    Shape nil_shape;
    CHECK(iface_cast<Areaer>(nil_shape) == nullptr && iface_cast<any>(nil_shape) == nullptr);
    CHECK(iface_cast<Empty>(any()) == nullptr);
    // The itab is found through the caches: converting again builds nothing and allocates nothing.
    const std::size_t builds = detail::itab_builds();
    const std::size_t cached = detail::itab_cached();
    const std::size_t before = allocations;
    int64_t sum = 0;
    for (int i = 0; i < 1000; i++) {
        sum += iface_cast<Areaer>(s).gp_tab() == a.gp_tab() ? 1 : 0;
    }
    const std::size_t after = allocations;
    CHECK(sum == 1000);
    CHECK(detail::itab_builds() == builds && detail::itab_cached() == cached);
    CHECK(after == before);
    // A moved source: the box moves, nothing is shared.
    Shape t = make_shape_rect_ptr(1, 1);
    const long uses = t.gp_data().use_count();
    Areaer moved = iface_cast<Areaer>(std::move(t));
    CHECK(t == nullptr && moved.gp_data().use_count() == uses);
    // The dynamic type decides, not the static type: alternating types through one target.
    Areaer c = iface_cast<Areaer>(gp::to_iface<Shape>(gp::new_<Rect>(Rect{1, 2}), td_pRect()));
    CHECK(c.Area() == 2);
    Namer nn = to_iface<Namer>(Rect{1, 2}, &gp_td_Rect);
    for (int i = 0; i < 4; i++) {
        Areaer x = (i % 2 == 0) ? iface_cast<Areaer>(nn) : iface_cast<Areaer>(s);
        CHECK(x.Area() == (i % 2 == 0 ? 2.0 : 6.0));
    }
}

// --- type assertions to a concrete type ---

void test_assert_type() {
    using namespace gp;
    any i = to_iface<any>(int64_t(42), &td_int);
    CHECK(assert_type<int64_t>(i, &td_int) == 42);
    CHECK(panic_text([&] { (void)assert_type<int64_t>(i, &td_int64); }) ==
          "interface conversion: interface {} is int, not int64");
    CHECK(panic_text([&] { (void)assert_type<gp::string>(i, &td_string); }) ==
          "interface conversion: interface {} is int, not string");
    // nil
    any nil;
    CHECK(panic_text([&] { (void)assert_type<int64_t>(nil, &td_int); }) ==
          "interface conversion: interface {} is nil, not int");
    // the static interface type's name
    Shape s = make_shape_rect_ptr(2, 3);
    CHECK(panic_text([&] { (void)assert_type<Rect>(s, &gp_td_Rect); }) ==
          "interface conversion: main.Shape is *main.Rect, not main.Rect");
    CHECK(panic_text([&] { (void)assert_type<Rect>(Shape(), &gp_td_Rect); }) ==
          "interface conversion: main.Shape is nil, not main.Rect");
    error e = to_iface<error>(gp::new_<MyErr>(MyErr{"boom"_gs}), td_pMyErr());
    CHECK(panic_text([&] { (void)assert_type<Rect>(e, &gp_td_Rect); }) ==
          "interface conversion: error is *main.MyErr, not main.Rect");
    // a pointer: the pointer shares the owner, nil pointers work
    ptr<Rect> p = assert_type<ptr<Rect>>(s, td_pRect());
    CHECK(p != nullptr && p->W == 2 && p.gp_shared() == s.gp_data());
    any np = to_iface<any>(ptr<Rect>(), td_pRect());
    CHECK(np != nullptr);
    CHECK(assert_type<ptr<Rect>>(np, td_pRect()) == nullptr);
    // a value: a reference into the box
    any r = to_iface<any>(Rect{4, 5}, &gp_td_Rect);
    const Rect& ref = assert_type<Rect>(r, &gp_td_Rect);
    CHECK(&ref == r.gp_data().get() && ref.H == 5);
    static_assert(std::is_same_v<decltype(assert_type<Rect>(r, &gp_td_Rect)), const Rect&>);
    static_assert(std::is_same_v<decltype(assert_type<ptr<Rect>>(r, td_pRect())), ptr<Rect>>);
    // a named type and its underlying type are different types
    any c = to_iface<any>(Celsius(36.6), &gp_td_Celsius);
    CHECK(assert_type<Celsius>(c, &gp_td_Celsius).gp_value() == 36.6);
    CHECK(panic_text([&] { (void)assert_type<double>(c, &td_float64); }) ==
          "interface conversion: interface {} is main.Celsius, not float64");
    // composite types
    any sl = to_iface<any>(gp::slice_of<int64_t>({1, 2}), td_slice<int64_t>(&td_int));
    CHECK(panic_text([&] {
              (void)assert_type<gp::slice<int64_t>>(sl, td_slice<int64_t>(&td_int64));
          }) == "interface conversion: interface {} is []int, not []int64");
    // the same name, other package or other scope
    any r2 = to_iface<any>(Rect2{1}, &gp_td_Rect2_pkg);
    CHECK(panic_text([&] { (void)assert_type<Rect2>(r2, &gp_td_Rect2_scope); }) ==
          "interface conversion: interface {} is main.Rect, not main.Rect (types from different "
          "packages)");
    any r3 = to_iface<any>(Rect2{1}, &gp_td_Rect2_scope);
    CHECK(panic_text([&] { (void)assert_type<Rect2>(r3, &gp_td_Rect2_pkg); }) ==
          "interface conversion: interface {} is main.Rect, not main.Rect (types from different "
          "packages)");
    CHECK(panic_text([&] { (void)assert_type<Rect>(r3, &gp_td_Rect); }) ==
          "interface conversion: interface {} is main.Rect, not main.Rect (types from different "
          "scopes)");
    // pointers to those types: the package is the named type's (Go's *T has T's package path)
    const type_desc* tp_pkg = td_ptr<Rect2>(&gp_td_Rect2_pkg);
    const type_desc* tp_scope = td_ptr<Rect2>(&gp_td_Rect2_scope);
    any pr = to_iface<any>(gp::new_<Rect2>(Rect2{1}), tp_pkg);
    CHECK(panic_text([&] { (void)assert_type<ptr<Rect2>>(pr, tp_scope); }) ==
          "interface conversion: interface {} is *main.Rect, not *main.Rect (types from different "
          "packages)");
    CHECK(panic_text([&] { (void)assert_type<ptr<Rect>>(pr, td_pRect()); }) ==
          "interface conversion: interface {} is *main.Rect, not *main.Rect (types from different "
          "packages)");
    any ps = to_iface<any>(gp::new_<Rect2>(Rect2{1}), tp_scope);
    CHECK(panic_text([&] { (void)assert_type<ptr<Rect>>(ps, td_pRect()); }) ==
          "interface conversion: interface {} is *main.Rect, not *main.Rect (types from different "
          "scopes)");
    // no allocation, no itab
    const std::size_t before = allocations;
    const std::size_t cached = detail::itab_cached();
    int64_t sum = 0;
    for (int k = 0; k < 1000; k++) {
        sum += assert_type<int64_t>(i, &td_int);
    }
    const std::size_t after = allocations;
    CHECK(sum == 42000 && after == before && detail::itab_cached() == cached);
}

void test_assert_type2() {
    using namespace gp;
    any i = to_iface<any>(int64_t(42), &td_int);
    auto [v, ok] = assert_type2<int64_t>(i, &td_int);
    CHECK(ok && v == 42);
    auto [w, ok2] = assert_type2<int64_t>(i, &td_int64);
    CHECK(!ok2 && w == 0);
    auto [s, ok3] = assert_type2<gp::string>(any(), &td_string);
    CHECK(!ok3 && s.gp_view().empty());
    auto [p, ok4] = assert_type2<ptr<Rect>>(make_shape_rect_ptr(1, 2), td_pRect());
    CHECK(ok4 && p->H == 2);
    auto [q, ok5] = assert_type2<ptr<Rect>>(i, td_pRect());
    CHECK(!ok5 && q == nullptr);
    auto [r, ok6] = assert_type2<Rect>(i, &gp_td_Rect);
    CHECK(!ok6 && r.W == 0 && r.H == 0);
}

// --- type assertions to an interface type ---

void test_assert_iface() {
    using namespace gp;
    any a = to_iface<any>(Rect{2, 3}, &gp_td_Rect);
    // Rect (a value) has Area and Name, not Scale.
    Areaer ar = assert_iface<Areaer>(a);
    CHECK(ar.Area() == 6 && ar.gp_type() == &gp_td_Rect);
    Namer nm = assert_iface<Namer>(a);
    CHECK(nm.Name() == "rect"_gs);
    CHECK(panic_text([&] { (void)assert_iface<Shape>(a); }) ==
          "interface conversion: main.Rect is not main.Shape: missing method Scale");
    // *Rect has them all
    any p = to_iface<any>(gp::new_<Rect>(Rect{1, 1}), td_pRect());
    Shape sh = assert_iface<Shape>(p);
    sh.Scale(3);
    CHECK(sh.Area() == 9);
    // nil: "interface is nil", whatever the static type is
    CHECK(panic_text([&] { (void)assert_iface<Shape>(any()); }) ==
          "interface conversion: interface is nil, not main.Shape");
    CHECK(panic_text([&] { (void)assert_iface<Shape>(Namer()); }) ==
          "interface conversion: interface is nil, not main.Shape");
    CHECK(panic_text([&] { (void)assert_iface<any>(any()); }) ==
          "interface conversion: interface is nil, not interface {}");
    // a type without the method
    any i = to_iface<any>(int64_t(1), &td_int);
    CHECK(panic_text([&] { (void)assert_iface<Namer>(i); }) ==
          "interface conversion: int is not main.Namer: missing method Name");
    CHECK(panic_text([&] { (void)assert_iface<error>(i); }) ==
          "interface conversion: int is not error: missing method Error");
    // the empty interface accepts everything that is not nil
    CHECK(assert_iface<Empty>(i).gp_type() == &td_int);
    // comma-ok
    auto [x, ok] = assert_iface2<Shape>(a);
    CHECK(!ok && x == nullptr);
    auto [y, ok2] = assert_iface2<Shape>(p);
    CHECK(ok2 && y.Area() == 9); // the same Rect, scaled above
    auto [z, ok3] = assert_iface2<Shape>(any());
    CHECK(!ok3 && z == nullptr);
    auto [w, ok4] = assert_iface2<Empty>(i);
    CHECK(ok4 && w != nullptr);
    auto [u, ok5] = assert_iface2<Empty>(any());
    CHECK(!ok5 && u == nullptr);
}

// --- type switches ---

void test_type_switch_helpers() {
    using namespace gp;
    any a = to_iface<any>(Rect{2, 3}, &gp_td_Rect);
    CHECK(type_is(a, &gp_td_Rect) && !type_is(a, td_pRect()) && !type_is(any(), &gp_td_Rect));
    CHECK(type_has<Areaer>(a) && type_has<Namer>(a) && !type_has<Shape>(a));
    CHECK(type_has<Empty>(a) && type_has<any>(a));
    CHECK(!type_has<Empty>(any()) && !type_has<Shape>(any()));
    CHECK(type_has<Shape>(make_shape_rect_ptr(1, 1)));
    // The one-entry caches of a miss and of a hit, asked in turn with other types: every answer is
    // right.
    any ptr_value = to_iface<any>(gp::new_<Rect>(Rect{1, 1}), td_pRect());
    any int_value = to_iface<any>(int64_t(1), &td_int);
    for (int i = 0; i < 3; i++) {
        CHECK(!type_has<Shape>(a) && !type_has<Shape>(a));
        CHECK(type_has<Shape>(ptr_value) && type_has<Shape>(ptr_value));
        CHECK(!type_has<Shape>(int_value) && !type_has<Shape>(a));
        CHECK(type_has<Areaer>(a) && type_has<Areaer>(ptr_value) && !type_has<Areaer>(int_value));
        CHECK(!std::get<1>(assert_iface2<Shape>(a)) &&
              std::get<1>(assert_iface2<Shape>(ptr_value)));
    }
    // a hit builds nothing and allocates nothing
    const std::size_t builds = detail::itab_builds();
    const std::size_t before = allocations;
    int hits = 0;
    for (int i = 0; i < 1000; i++) {
        hits += type_has<Areaer>(a) ? 1 : 0;
    }
    const std::size_t after = allocations;
    CHECK(hits == 1000 && detail::itab_builds() == builds && after == before);
}

// --- gp::error ---

// panic_type runs f and returns the name of the dynamic type of the value of the panic it throws,
// and whether Error() of that value is the panic's text.
template <typename F>
std::string panic_type(F f) {
    try {
        f();
    } catch (const gp::panic_error& e) {
        if (e.gp_value == nullptr) {
            return "<nil value>";
        }
        std::string name = e.gp_value.gp_type()->gp_name;
        gp::error err = gp::iface_cast<gp::error>(e.gp_value);
        return name + (err.Error() == e.gp_text ? "" : " (Error() differs)");
    }
    return "<no panic>";
}

// A failed assertion panics with a *runtime.TypeAssertionError value (recover() gives it; %T prints
// the type), in every variant.
void test_assert_panic_value() {
    using namespace gp;
    const std::string want = "*runtime.TypeAssertionError";
    any i = to_iface<any>(int64_t(1), &td_int);
    CHECK(panic_type([&] { (void)assert_type<gp::string>(i, &td_string); }) == want);
    CHECK(panic_type([&] { (void)assert_type<gp::string>(any(), &td_string); }) == want);
    CHECK(panic_type([&] { (void)assert_iface<Namer>(i); }) == want);
    CHECK(panic_type([&] { (void)assert_iface<Namer>(any()); }) == want);
    CHECK(panic_type([&] { (void)assert_type<Rect>(make_shape_rect_ptr(1, 1), &gp_td_Rect); }) ==
          want);
    // Not an assertion: other panics keep their own types.
    CHECK(panic_type([&] {
              (void)(to_iface<any>(gp::slice_of<int64_t>({1}), td_slice<int64_t>(&td_int)) ==
                     to_iface<any>(gp::slice_of<int64_t>({1}), td_slice<int64_t>(&td_int)));
          }) != want);
}

void test_method_recv() {
    using namespace gp;
    CHECK(panic_text([&] { (void)method_recv(Shape()); }) ==
          "runtime error: invalid memory address or nil pointer dereference");
    Shape s = make_shape_rect_ptr(2, 3);
    Shape t = method_recv(s);
    CHECK(t.gp_data() == s.gp_data() && t.Area() == 6);
    CHECK(method_recv(make_shape_rect_ptr(1, 1)).Area() == 1);
}

void test_error() {
    using namespace gp;
    static_assert(sizeof(error) == 32);
    CHECK(std::string(td_error.gp_name) == "error" && td_error.gp_kind == kind::Interface);
    CHECK(td_error.gp_named && td_error.gp_pkg == nullptr && td_error.gp_nimethods == 1);
    error nil;
    CHECK(nil == nullptr);
    CHECK(panic_text([&] { (void)nil.Error(); }) ==
          "runtime error: invalid memory address or nil pointer dereference");
    error e = to_iface<error>(gp::new_<MyErr>(MyErr{"boom"_gs}), td_pMyErr());
    CHECK(e != nullptr && e.Error() == "boom"_gs);
    // an error converted to any and back by an assertion
    any a = iface_cast<any>(e);
    error back = assert_iface<error>(a);
    CHECK(back.Error() == "boom"_gs && back.gp_data() == e.gp_data());
    ptr<MyErr> p = assert_type<ptr<MyErr>>(e, td_pMyErr());
    CHECK(p->msg == "boom"_gs);
    // a nil *MyErr in an error is not nil (the nil gotcha)
    error typed_nil = to_iface<error>(ptr<MyErr>(), td_pMyErr());
    CHECK(typed_nil != nullptr);
    CHECK(panic_text([&] { (void)assert_type<MyErr>(e, &gp_td_Rect); }) ==
          "interface conversion: error is *main.MyErr, not main.Rect");
}

// --- == and the hash of interface values ---

struct Uncomparable {
    gp::slice<int64_t> s;
};

void test_equal_hash() {
    using namespace gp;
    any a = to_iface<any>(int64_t(1), &td_int);
    any b = to_iface<any>(int64_t(1), &td_int);
    any c = to_iface<any>(int64_t(1), &td_int64);
    any d = to_iface<any>(int64_t(2), &td_int);
    CHECK(a == b && !(a != b));
    CHECK(a != c && a != d); // another Go type, another value
    CHECK(any() == any() && a != any() && any() != a);
    CHECK(a.gp_hash() == b.gp_hash());
    CHECK(any().gp_hash() == any().gp_hash());
    // strings, structs, pointers
    CHECK(to_iface<any>("x"_gs, &td_string) == to_iface<any>("x"_gs, &td_string));
    CHECK(to_iface<any>("x"_gs, &td_string) != to_iface<any>("y"_gs, &td_string));
    CHECK(to_iface<any>(Rect{1, 2}, &gp_td_Rect) == to_iface<any>(Rect{1, 2}, &gp_td_Rect));
    CHECK(to_iface<any>(Rect{1, 2}, &gp_td_Rect) != to_iface<any>(Rect{1, 3}, &gp_td_Rect));
    CHECK(to_iface<any>(Rect{1, 2}, &gp_td_Rect).gp_hash() ==
          to_iface<any>(Rect{1, 2}, &gp_td_Rect).gp_hash());
    ptr<Rect> p = gp::new_<Rect>(Rect{1, 1});
    ptr<Rect> q = gp::new_<Rect>(Rect{1, 1});
    any ap1 = to_iface<any>(p, td_pRect());
    any ap2 = to_iface<any>(p, td_pRect());
    any aq = to_iface<any>(q, td_pRect());
    CHECK(ap1 == ap2 && ap1 != aq && ap1.gp_hash() == ap2.gp_hash());
    // a nil pointer in an interface is equal to another one, and not to nil
    any n1 = to_iface<any>(ptr<Rect>(), td_pRect());
    any n2 = to_iface<any>(ptr<Rect>(), td_pRect());
    CHECK(n1 == n2 && n1 != any() && n1.gp_hash() == n2.gp_hash());
    // different interface types compare by their dynamic values
    Shape s1 = make_shape_rect_ptr(1, 1);
    any as = iface_cast<any>(s1);
    CHECK(as == s1 && s1 == as);
    // uncomparable dynamic types panic when both have the type, not when they differ
    static const type_desc td_unc =
        named<Uncomparable>("main.Unc", "main", shape_struct(nullptr, 0));
    any u1 = to_iface<any>(Uncomparable{}, &td_unc);
    any u2 = to_iface<any>(Uncomparable{}, &td_unc);
    CHECK(panic_text([&] { (void)(u1 == u2); }) ==
          "runtime error: comparing uncomparable type main.Unc");
    CHECK(panic_text([&] { (void)u1.gp_hash(); }) ==
          "runtime error: hash of unhashable type main.Unc");
    CHECK(panic_text([&] { return u1 == a; }) == "<no panic>");
    any sl1 = to_iface<any>(gp::slice_of<int64_t>({1}), td_slice<int64_t>(&td_int));
    CHECK(panic_text([&] { (void)(sl1 == sl1); }) ==
          "runtime error: comparing uncomparable type []int");
    CHECK(panic_text([&] { (void)sl1.gp_hash(); }) ==
          "runtime error: hash of unhashable type []int");

    // The hash spreads: distinct values give mostly distinct hashes, and equal values equal ones
    // (also 0.0 and -0.0, which == calls equal).
    std::set<std::size_t> hashes;
    for (int64_t i = 0; i < 200; i++) {
        hashes.insert(to_iface<any>(i, &td_int).gp_hash());
    }
    CHECK(hashes.size() > 190);
    hashes.clear();
    for (int i = 0; i < 100; i++) {
        hashes.insert(
            to_iface<any>(gp::string::gp_copy("key" + std::to_string(i)), &td_string).gp_hash());
    }
    CHECK(hashes.size() > 95);
    any pz = to_iface<any>(0.0, &td_float64);
    any nz = to_iface<any>(-0.0, &td_float64);
    CHECK(pz == nz && pz.gp_hash() == nz.gp_hash());
    // A key check of a read in an empty map: Go's own text, without "runtime error: ".
    CHECK(panic_text([&] { gp::check_key(sl1); }) == "hash of unhashable type: []int");
    CHECK(panic_text([&] { gp::check_key(a); }) == "<no panic>");
    CHECK(panic_text([&] { gp::check_key(any()); }) == "<no panic>");
    auto empty = gp::make_map<any, int64_t>();
    CHECK(panic_text([&] { (void)gp::get(empty, sl1); }) == "hash of unhashable type: []int");
    CHECK(panic_text([&] { (void)gp::lookup(empty, sl1); }) == "hash of unhashable type: []int");
    CHECK(panic_text([&] { gp::erase(empty, sl1); }) == "hash of unhashable type: []int");
    CHECK(panic_text([&] { (void)gp::get(gp::map<any, int64_t>(), sl1); }) ==
          "hash of unhashable type: []int");
    CHECK(panic_text([&] { gp::set(empty, sl1, int64_t(1)); }) ==
          "runtime error: hash of unhashable type []int");
    gp::set(empty, a, int64_t(1));
    CHECK(panic_text([&] { (void)gp::get(empty, sl1); }) ==
          "runtime error: hash of unhashable type []int"); // not empty: the hash panics
    // C++ values: a type with == and no std::hash is still a key (one hash for all its values);
    // a raw pointer is held as the gp::cptr of its pointee, as a Go variable of type *T is.
    struct NoHash {
        int n = 0;
        bool operator==(const NoHash&) const = default;
    };
    any h1 = to_iface_foreign<any>(NoHash{1});
    any h2 = to_iface_foreign<any>(NoHash{1});
    any h3 = to_iface_foreign<any>(NoHash{2});
    CHECK(h1 == h2 && h1 != h3 && h1.gp_hash() == h2.gp_hash());
    auto km = gp::make_map<any, int64_t>();
    gp::set(km, h1, int64_t(10));
    gp::set(km, h3, int64_t(30));
    CHECK(gp::get(km, h2) == 10 && gp::get(km, h3) == 30 && gp::len(km) == 2);
    int cint = 5;
    any raw = to_iface_foreign<any>(&cint);
    any viaptr = to_iface_foreign<any>(cptr<int>(&cint));
    CHECK(raw.gp_type() == foreign_td<cptr<int>>() && raw.gp_type() == viaptr.gp_type());
    CHECK(raw == viaptr && *assert_type<cptr<int>>(raw, foreign_td<cptr<int>>()) == 5);

    // as map keys
    auto m = gp::make_map<any, int64_t>();
    gp::set(m, a, int64_t(10));
    gp::set(m, c, int64_t(20));
    gp::set(m, to_iface<any>("k"_gs, &td_string), int64_t(30));
    CHECK(gp::get(m, b) == 10 && gp::get(m, c) == 20 && gp::len(m) == 3);
    CHECK(panic_text([&] { gp::set(m, sl1, int64_t(1)); }) ==
          "runtime error: hash of unhashable type []int");
}

// A struct key with an interface field, as the emitter writes it (typedecl.go: gp_hash and
// gp_hash_panics next to ==), and the same inside an array and a named array type.
struct IKey {
    int64_t id = 0;
    gp::any val;
    bool operator==(const IKey&) const = default;
    std::size_t gp_hash() const { return gp::hash_fields(id, val); }
    static constexpr bool gp_hash_panics = true;
};
struct PlainKey {
    int64_t id = 0;
    bool operator==(const PlainKey&) const = default;
    std::size_t gp_hash() const { return gp::hash_fields(id); }
};
struct NamedIA : gp::newtype<NamedIA, gp::array<gp::any, 2>> {
    using gp::newtype<NamedIA, gp::array<gp::any, 2>>::newtype;
};

// gp::check_key (Go's mapKeyError) reaches into structs, arrays and named arrays.
void test_key_check() {
    using namespace gp;
    static_assert(detail::hash_may_panic<any> && detail::hash_may_panic<IKey>);
    static_assert(detail::hash_may_panic<array<any, 2>> && detail::hash_may_panic<array<IKey, 1>>);
    static_assert(detail::hash_may_panic<NamedIA>);
    static_assert(!detail::hash_may_panic<PlainKey> && !detail::hash_may_panic<int64_t> &&
                  !detail::hash_may_panic<array<int64_t, 3>> &&
                  !detail::hash_may_panic<array<any, 0>>);
    any sl = to_iface<any>(gp::slice_of<int64_t>({1}), td_slice<int64_t>(&td_int));
    any ok = to_iface<any>(int64_t(1), &td_int);
    const char* want = "hash of unhashable type: []int";
    CHECK(panic_text([&] { gp::check_key(IKey{1, sl}); }) == want);
    CHECK(panic_text([&] { gp::check_key(IKey{1, ok}); }) == "<no panic>");
    CHECK(panic_text([&] { gp::check_key(IKey{1, any()}); }) == "<no panic>");
    array<any, 2> arr;
    arr[1] = sl;
    CHECK(panic_text([&] { gp::check_key(arr); }) == want);
    arr[1] = ok;
    CHECK(panic_text([&] { gp::check_key(arr); }) == "<no panic>");
    array<IKey, 1> ak{IKey{2, sl}};
    CHECK(panic_text([&] { gp::check_key(ak); }) == want);
    NamedIA na{array<any, 2>{ok, sl}};
    CHECK(panic_text([&] { gp::check_key(na); }) == want);
    // An interface that holds a struct with an interface field that holds the slice.
    static const type_desc td_ikey = named<IKey>("main.IKey", "main", shape_struct(nullptr, 0));
    any boxed = to_iface<any>(IKey{1, sl}, &td_ikey);
    CHECK(panic_text([&] { gp::check_key(boxed); }) == want);
    CHECK(panic_text([&] { gp::check_key(to_iface<any>(IKey{1, ok}, &td_ikey)); }) == "<no panic>");
    // The mode is left again after a panic: the plain hash has the other text.
    CHECK(panic_text([&] { (void)IKey{1, sl}.gp_hash(); }) ==
          "runtime error: hash of unhashable type []int");
    CHECK(!detail::key_checking);
    // In maps: read and delete of a nil and an empty map check, a filled map hashes.
    auto empty = gp::make_map<IKey, int64_t>();
    CHECK(panic_text([&] { (void)gp::get(empty, IKey{1, sl}); }) == want);
    CHECK(panic_text([&] { gp::erase(empty, IKey{1, sl}); }) == want);
    CHECK(panic_text([&] { (void)gp::lookup(gp::map<IKey, int64_t>(), IKey{1, sl}); }) == want);
    CHECK(panic_text([&] { (void)gp::get(empty, IKey{1, ok}); }) == "<no panic>");
    CHECK(panic_text([&] { gp::set(empty, IKey{1, sl}, int64_t(1)); }) ==
          "runtime error: hash of unhashable type []int");
    gp::set(empty, IKey{1, ok}, int64_t(7));
    CHECK(gp::get(empty, IKey{1, ok}) == 7);
    CHECK(panic_text([&] { (void)gp::get(empty, IKey{1, sl}); }) ==
          "runtime error: hash of unhashable type []int");
    auto arrmap = gp::make_map<array<any, 2>, int64_t>();
    CHECK(panic_text([&] { (void)gp::get(arrmap, arr = array<any, 2>{sl, ok}); }) == want);
}

// C/C++ pointers in interfaces: const and nesting are normalised on both sides.
void test_foreign_norm() {
    using namespace gp;
    struct Pt {
        int x, y;
    };
    static_assert(std::is_same_v<detail::foreign_norm_t<const char*>, char*>);
    static_assert(std::is_same_v<detail::foreign_norm_t<const Pt* const*>, Pt**>);
    static_assert(std::is_same_v<detail::foreign_norm_t<Pt* const* const>, Pt**>);
    static_assert(std::is_same_v<detail::foreign_norm_t<cptr<const Pt>>, cptr<Pt>>);
    static_assert(std::is_same_v<detail::foreign_norm_t<const volatile int>, int>);
    static_assert(std::is_same_v<detail::foreign_norm_t<int (*)(int)>, int (*)(int)>);
    static_assert(std::is_same_v<detail::foreign_norm_t<const void*>, void*>);
    CHECK(foreign_td<const char*>() == foreign_td<char*>());
    CHECK(foreign_td<cptr<const Pt>>() == foreign_td<cptr<Pt>>());
    CHECK(foreign_td<Pt**>() == foreign_td<const Pt* const*>());
    CHECK(foreign_td<Pt*>() != foreign_td<Pt**>());

    Pt pt{3, 4};
    const Pt* cp = &pt;
    Pt* mp = &pt;
    any a = to_iface_foreign<any>(cp);
    any b = to_iface_foreign<any>(mp);
    CHECK(a.gp_type() == b.gp_type() && a.gp_type() == foreign_td<cptr<Pt>>() && a == b &&
          a.gp_hash() == b.gp_hash());
    CHECK(std::get<1>(assert_type2<cptr<Pt>>(a, foreign_td<cptr<Pt>>())));
    Pt* pp = &pt;
    Pt** ppp = &pp;
    const Pt* const* cppp = ppp;
    any c = to_iface_foreign<any>(ppp);
    any d = to_iface_foreign<any>(cppp);
    CHECK(c.gp_type() == foreign_td<Pt**>() && c.gp_type() == d.gp_type() && c == d);
    CHECK(c.gp_type() != a.gp_type());
    CHECK(assert_type<Pt**>(c, foreign_td<Pt**>()) == ppp);
    const char* name = "hello";
    any e = to_iface_foreign<any>(name);
    CHECK(e.gp_type() == foreign_td<cptr<char>>());
    CHECK(std::get<1>(assert_type2<cptr<char>>(e, foreign_td<cptr<char>>())));
    auto m = gp::make_map<any, int64_t>();
    gp::set(m, a, int64_t(1));
    CHECK(gp::get(m, b) == 1 && gp::len(m) == 1);
}

// The spread of the hash of interface values: the low bits (the bucket of a table) are used
// evenly for sequential integers, aligned pointers, strings and struct keys, so the fullest
// bucket of 1024 holds a small multiple of the mean (8) for 8192 keys.
template <typename F>
std::size_t fullest_bucket(F make, int n) {
    std::size_t counts[1024] = {};
    for (int i = 0; i < n; i++) {
        counts[make(i).gp_hash() & 1023]++;
    }
    std::size_t most = 0;
    for (std::size_t c : counts) {
        most = c > most ? c : most;
    }
    return most;
}

void test_hash_spread() {
    using namespace gp;
    const int n = 8192;
    CHECK(fullest_bucket([](int i) { return to_iface<any>(int64_t(i), &td_int); }, n) <= 28);
    CHECK(fullest_bucket([](int i) { return to_iface<any>(int64_t(i) * 1024, &td_int); }, n) <= 28);
    CHECK(fullest_bucket([](int i) { return to_iface<any>(double(i) / 4, &td_float64); }, n) <= 28);
    CHECK(fullest_bucket(
              [](int i) {
                  return to_iface<any>(gp::string::gp_copy("key-" + std::to_string(i)), &td_string);
              },
              n) <= 28);
    CHECK(fullest_bucket(
              [](int i) { return to_iface<any>(Rect{double(i), double(-i)}, &gp_td_Rect); }, n) <=
          28);
    CHECK(
        fullest_bucket(
            [](int i) { return to_iface<any>(Rect{double(i % 64), double(i / 64)}, &gp_td_Rect); },
            n) <= 28);
    // Pointers 16 bytes apart (the descriptor hashes the target address).
    static char block[8192 * 16];
    std::size_t counts[1024] = {};
    for (int i = 0; i < n; i++) {
        counts[detail::ptr_target_hash(block + i * 16) & 1023]++;
    }
    std::size_t most = 0;
    for (std::size_t c : counts) {
        most = c > most ? c : most;
    }
    CHECK(most <= 28);
    // Equal numbers of different types do collide on purpose (the == tells them apart): a map
    // of the same number as int and int64 and uint8 has three entries and finds each.
    auto m = gp::make_map<any, int64_t>();
    gp::set(m, to_iface<any>(int64_t(1), &td_int), int64_t(1));
    gp::set(m, to_iface<any>(int64_t(1), &td_int64), int64_t(2));
    gp::set(m, to_iface<any>(uint8_t(1), &td_uint8), int64_t(3));
    CHECK(gp::len(m) == 3 && gp::get(m, to_iface<any>(int64_t(1), &td_int64)) == 2);
}

} // namespace

// Counting operator new: an assertion or a conversion allocates nothing.
// (g++ -O2 inlines the pairs of new and delete of the runtime's own tables into these and warns
// that malloc'ed memory is freed by free: it is not a mismatch.)
#pragma GCC diagnostic push
#pragma GCC diagnostic ignored "-Wmismatched-new-delete"
void* operator new(std::size_t n) {
    allocations++;
    if (void* p = std::malloc(n == 0 ? 1 : n)) {
        return p;
    }
    throw std::bad_alloc();
}
void operator delete(void* p) noexcept {
    std::free(p);
}
void operator delete(void* p, std::size_t) noexcept {
    std::free(p);
}
#pragma GCC diagnostic pop

int main() {
    test_iface_cast();
    test_assert_type();
    test_assert_type2();
    test_assert_iface();
    test_type_switch_helpers();
    test_error();
    test_method_recv();
    test_assert_panic_value();
    test_equal_hash();
    test_key_check();
    test_foreign_norm();
    test_hash_spread();
    if (failures != 0) {
        return 1;
    }
    std::string msg = "ok: " + std::to_string(checks) + " checks\n";
    gp::os::write(gp::os::stdout_fd, msg.data(), msg.size());
    return 0;
}
