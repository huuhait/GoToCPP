// The unit tests of interface values and itabs (gocpp/iface.hpp, Phase 05 steps 5.2 and 5.3):
// the nil interface, the boxing of small and large values, pointers shared with their owner,
// a nil pointer in an interface (not nil), copies and moves, destruction exactly once, a long
// chain of interfaces that hold interfaces, itabs (method sets of T and *T, signature
// texts, unexported names of two packages, opaque C types, the caches: a hit builds nothing,
// a miss is cached too), the static conversion with its per-site cache, gp::icall and
// icall_keep, a nil interface's method, foreign values in an interface, and getitab from many
// threads at once. The types are written the way the emitter will write them (the shapes in
// the comment of gocpp/iface.hpp). runtime/cxx_test.go compiles this file with the runtime's
// .cpp files, with g++ and clang++, at -O0, at -O2 and with ASan + UBSan (LeakSanitizer
// included), and runs it like test/unit.cpp. It prints "ok: N checks" and exits 0, or prints
// each failed check and exits 1.
#include <gocpp/rt.hpp>
#include <gocpp/os.hpp>

#include <cmath>
#include <cstdint>
#include <deque>
#include <memory>
#include <set>
#include <string>
#include <thread>
#include <tuple>
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
//     type Shape interface { Area() float64; Scale(k float64) }
//     type Areaer interface { Area() float64 }
//     type Namer interface { Name() string }
//     type Rect struct{ W, H float64 }
//     func (r Rect) Area() float64 { ... }          value methods: Area, Name
//     func (r Rect) Name() string { ... }
//     func (r *Rect) Scale(k float64) { ... }       a pointer method
//     type Circle struct{ R float64 }
//     func (c *Circle) Area() float64 { ... }       only *Circle has it

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

const gp::type_desc* shape_desc();

struct Shape : gp::iface {
    using gp::iface::iface;
    static const gp::type_desc* gp_desc() { return shape_desc(); }
    double Area() const;
    void Scale(double k) const;
};

const gp::type_desc* areaer_desc();

struct Areaer : gp::iface {
    using gp::iface::iface;
    static const gp::type_desc* gp_desc() { return areaer_desc(); }
    double Area() const;
};

const gp::type_desc* namer_desc();

struct Namer : gp::iface {
    using gp::iface::iface;
    static const gp::type_desc* gp_desc() { return namer_desc(); }
    gp::string Name() const;
};

static_assert(sizeof(Shape) == 32 && sizeof(Areaer) == 32 && sizeof(Namer) == 32);

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

// The thunks: for Rect (an interface that holds a Rect box) and *Rect (a pointer).
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

// A thunk with the wrong C++ type for Area() float64.
[[maybe_unused]] int64_t thunk_wrong(const gp::iface&) {
    return 0;
}

// The method tables, sorted.
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

const gp::imethod gp_im_Shape[] = {gp::imethod("Area", nullptr, "func() float64"),
                                   gp::imethod("Scale", nullptr, "func(float64)")};
const gp::imethod gp_im_Areaer[] = {gp::imethod("Area", nullptr, "func() float64")};
const gp::imethod gp_im_Namer[] = {gp::imethod("Name", nullptr, "func() string")};

const gp::type_desc gp_td_Rect =
    gp::named<Rect>("main.Rect", "main", gp::shape_struct(nullptr, 0), gp::methods_of(gp_vm_Rect),
                    gp::methods_of(gp_pm_Rect));
const gp::type_desc gp_td_Circle = gp::named<Circle>(
    "main.Circle", "main", gp::shape_struct(nullptr, 0), {}, gp::methods_of(gp_pm_Circle));
const gp::type_desc gp_td_Shape =
    gp::named<Shape>("main.Shape", "main", gp::shape_interface(gp_im_Shape));
const gp::type_desc gp_td_Areaer =
    gp::named<Areaer>("main.Areaer", "main", gp::shape_interface(gp_im_Areaer));
const gp::type_desc gp_td_Namer =
    gp::named<Namer>("main.Namer", "main", gp::shape_interface(gp_im_Namer));

const gp::type_desc* shape_desc() {
    return &gp_td_Shape;
}
const gp::type_desc* areaer_desc() {
    return &gp_td_Areaer;
}
const gp::type_desc* namer_desc() {
    return &gp_td_Namer;
}

const gp::type_desc* td_pRect() {
    static const gp::type_desc* const td = gp::td_ptr<Rect>(&gp_td_Rect);
    return td;
}
const gp::type_desc* td_pCircle() {
    static const gp::type_desc* const td = gp::td_ptr<Circle>(&gp_td_Circle);
    return td;
}

// What a static conversion site asserts for every method of the type (the checks of a method
// table's entries are in method_of).
static_assert(gp::thunk_matches<double(), decltype(&thunk_Rect_Area)>);
static_assert(gp::thunk_matches<void(double), decltype(&thunk_pRect_Scale)>);
static_assert(gp::thunk_matches<gp::string(), decltype(&thunk_Rect_Name)>);
static_assert(!gp::thunk_matches<double(), decltype(&thunk_wrong)>);
static_assert(!gp::thunk_matches<void(double), decltype(&thunk_Rect_Area)>);
static_assert(!gp::thunk_matches<double(double), decltype(&thunk_Rect_Area)>);
static_assert(gp::thunk_matches<double(), double(const gp::iface&)>);

// --- the nil interface ---

void test_nil() {
    gp::any a;
    CHECK(a == nullptr);
    CHECK(a.gp_type() == nullptr && a.gp_tab() == nullptr && a.gp_data() == nullptr);
    CHECK(!(a != nullptr));
    CHECK(gp::any(nullptr) == nullptr);
    Shape s;
    CHECK(s == nullptr && s.gp_type() == nullptr);
    Shape s2(nullptr);
    CHECK(s2 == nullptr);
    gp::any copy = a;
    CHECK(copy == nullptr);
    // Every interface converts to any, nil too.
    gp::any from_shape = s;
    CHECK(from_shape == nullptr);
    // A method of a nil interface panics like Go's nil dereference.
    const std::string nil_msg = "runtime error: invalid memory address or nil pointer dereference";
    CHECK(panic_text([&] { (void)s.Area(); }) == nil_msg);
    CHECK(panic_text([&] { s.Scale(2); }) == nil_msg);
    CHECK(panic_text([&] { (void)gp::icall<double()>(s, 0); }) == nil_msg);
    CHECK(panic_text([&] { (void)gp::icall_keep<double()>(s, 0); }) == nil_msg);
    // Moving leaves nil behind.
    gp::any b = gp::to_iface<gp::any>(int64_t(1), &gp::td_int);
    CHECK(b != nullptr);
    gp::any c = std::move(b);
    CHECK(b == nullptr && b.gp_data() == nullptr && c != nullptr);
    gp::any d;
    d = std::move(c);
    CHECK(c == nullptr && d != nullptr);
    gp::any& alias = d;
    d = std::move(alias); // a self-move leaves it as it is
    CHECK(d != nullptr);
}

// --- boxing ---

void test_small_values() {
    using namespace gp;
    // int and int64 are both int64_t in C++: the descriptor says which Go type it is.
    any i = to_iface<any>(int64_t(42), &td_int);
    any i64 = to_iface<any>(int64_t(42), &td_int64);
    CHECK(i.gp_type() == &td_int && i64.gp_type() == &td_int64 && i.gp_type() != i64.gp_type());
    CHECK(unbox<int64_t>(i) == 42 && unbox<int64_t>(i64) == 42);
    CHECK(i.gp_tab() == nullptr); // an empty interface has no itab
    CHECK(i != nullptr);
    // Each value has a box of its own, a copy: the original can change.
    int64_t n = 7;
    any boxed = to_iface<any>(n, &td_int);
    n = 8;
    CHECK(unbox<int64_t>(boxed) == 7);
    // Other small values.
    CHECK(unbox<bool>(to_iface<any>(true, &td_bool)) == true);
    CHECK(unbox<uint8_t>(to_iface<any>(uint8_t(200), &td_uint8)) == 200);
    CHECK(unbox<float>(to_iface<any>(1.5f, &td_float32)) == 1.5f);
    CHECK(unbox<double>(to_iface<any>(2.5, &td_float64)) == 2.5);
    CHECK(unbox<gp::complex128>(to_iface<any>(gp::complex128(1, 2), &td_complex128)) ==
          gp::complex128(1, 2));
    CHECK(unbox<gp::string>(to_iface<any>("hello"_gs, &td_string)) == "hello"_gs);
    gp::string heap = gp::string::gp_copy(std::string(100, 'x'));
    CHECK(unbox<gp::string>(to_iface<any>(heap, &td_string)) == heap);
    CHECK(unbox<Rect>(to_iface<Areaer>(Rect{2, 3}, &gp_td_Rect)).W == 2);
    // The value of the box is an object of the C++ type, by reference.
    any r = to_iface<any>(Rect{4, 5}, &gp_td_Rect);
    const Rect& ref = unbox<Rect>(r);
    CHECK(&ref == r.gp_data().get() && ref.H == 5);
    // An unbox of a temporary is a copy.
    Rect copy = unbox<Rect>(to_iface<any>(Rect{6, 7}, &gp_td_Rect));
    CHECK(copy.W == 6 && copy.H == 7);
    static_assert(std::is_same_v<decltype(unbox<Rect>(r)), const Rect&>);
    static_assert(std::is_same_v<decltype(unbox<Rect>(to_iface<any>(Rect{}, &gp_td_Rect))), Rect>);
}

struct Big {
    int64_t a[8192]; // 64 KiB
};

void test_large_values() {
    using namespace gp;
    static const type_desc td_Big = named<Big>("main.Big", "main", shape_struct(nullptr, 0));
    auto big = std::make_unique<Big>();
    for (int i = 0; i < 8192; i++) {
        big->a[i] = i * 3;
    }
    any x = to_iface<any>(*big, &td_Big);
    big->a[5] = -1; // the box has its own copy
    const Big& got = unbox<Big>(x);
    CHECK(got.a[5] == 15 && got.a[8191] == 8191 * 3 && x.gp_type() == &td_Big);
    // Copies of the interface share the one box.
    any y = x;
    CHECK(&unbox<Big>(y) == &got);
    CHECK(x.gp_data().use_count() == 2);
    y = gp::any();
    CHECK(x.gp_data().use_count() == 1);
    // A slice, a map, an array, a func in an interface: boxed like any other value.
    gp::slice<int64_t> s = gp::slice_of<int64_t>({1, 2, 3});
    any as = to_iface<any>(s, td_slice<int64_t>(&td_int));
    CHECK(unbox<gp::slice<int64_t>>(as).gp_data() == s.gp_data()); // shares the array
    CHECK(as.gp_type() == td_slice<int64_t>(&td_int));
    gp::array<int64_t, 3> arr{1, 2, 3};
    CHECK((unbox<gp::array<int64_t, 3>>(to_iface<any>(arr, td_array<int64_t, 3>(&td_int))) == arr));
    gp::func<int64_t()> f([]() -> int64_t { return 9; });
    CHECK(unbox<gp::func<int64_t()>>(
              to_iface<any>(f, td_func<gp::func<int64_t()>>({}, {&td_int})))() == 9);
}

// --- pointers ---

struct Counted {
    static inline int made = 0;
    static inline int copies = 0;
    static inline int destroyed = 0;
    int64_t id = 0;
    Counted() { made++; }
    explicit Counted(int64_t i) : id(i) { made++; }
    Counted(const Counted& o) : id(o.id) {
        made++;
        copies++;
    }
    Counted(Counted&& o) noexcept : id(o.id) { made++; }
    ~Counted() { destroyed++; }
    int live() const { return made - destroyed; }
};

void test_pointers() {
    using namespace gp;
    ptr<Rect> p = new_<Rect>(Rect{2, 3});
    CHECK(p.gp_shared().use_count() == 1);
    any x = to_iface<any>(p, td_pRect());
    CHECK(x.gp_type() == td_pRect() && x != nullptr);
    CHECK(p.gp_shared().use_count() == 2); // shared: the object isn't copied
    ptr<Rect> q = unbox<ptr<Rect>>(x);
    CHECK(q == p && q.gp_get() == p.gp_get());
    CHECK(p.gp_shared().use_count() == 3);
    q->W = 10; // the same object
    CHECK(p->W == 10);
    CHECK(x.gp_data().get() == p.gp_get());
    // The object lives as long as the interface does.
    static const type_desc td_Counted =
        named<Counted>("main.Counted", "main", shape_struct(nullptr, 0));
    const type_desc* td_pCounted = td_ptr<Counted>(&td_Counted);
    Counted::made = Counted::copies = Counted::destroyed = 0;
    {
        any held;
        {
            ptr<Counted> c = new_<Counted>(int64_t(5));
            held = to_iface<any>(c, td_pCounted);
        }
        CHECK(Counted::destroyed == 0);
        CHECK(unbox<ptr<Counted>>(held)->id == 5);
        CHECK(Counted::copies == 0);
    }
    CHECK(Counted::destroyed == 1 && Counted::made == 1);

    // A pointer from an rvalue moves its owner in.
    {
        ptr<Rect> r = new_<Rect>();
        Rect* addr = r.gp_get();
        any held = to_iface<any>(std::move(r), td_pRect());
        CHECK(r == nullptr && held.gp_data().get() == addr && held.gp_data().use_count() == 1);
    }
    // An interior pointer: &r.W shares r's block, and the interface holds that address.
    {
        ptr<Rect> r = new_<Rect>(Rect{1, 2});
        ptr<double> w(r, &r->H);
        any held = to_iface<any>(w, td_ptr<double>(&td_float64));
        r = nullptr; // the interface still keeps the whole Rect alive
        ptr<double> back = unbox<ptr<double>>(held);
        CHECK(back == w && *back == 2 && held.gp_data().get() == static_cast<void*>(w.gp_get()));
        *back = 20;
        CHECK(*w == 20);
    }
    // A pointer to a package variable (the static owner).
    static Rect global{1, 1};
    any g = to_iface<any>(static_ptr(global), td_pRect());
    CHECK(unbox<ptr<Rect>>(g).gp_get() == &global);
    // unsafe.Pointer.
    {
        int v = 0;
        ptr<void> up(static_owner(), &v);
        any u = to_iface<any>(up, &td_unsafe_pointer);
        CHECK(u.gp_type() == &td_unsafe_pointer && unbox<ptr<void>>(u).gp_get() == &v);
    }
}

// Go's famous gotcha: a nil pointer in an interface is not a nil interface.
void test_nil_pointer_in_interface() {
    using namespace gp;
    ptr<Rect> nilp;
    any x = to_iface<any>(nilp, td_pRect());
    CHECK(x != nullptr);              // x == nil is false
    CHECK(x.gp_type() == td_pRect()); // the type is *Rect
    CHECK(x.gp_data() == nullptr);    // and the pointer is nil
    CHECK(unbox<ptr<Rect>>(x) == nullptr);
    CHECK(unbox<ptr<Rect>>(x).gp_get() == nullptr);
    // The same through an interface with methods: its method is called with a nil *Rect, and
    // the value method's wrapper panics as Go's does; a pointer method gets the nil.
    Shape s = to_iface<Shape>(nilp, td_pRect());
    CHECK(s != nullptr && s.gp_tab() != nullptr);
    CHECK(panic_text([&] { (void)s.Area(); }) ==
          "value method main.Rect.Area called using nil *Rect pointer");
    CHECK(panic_text([&] { s.Scale(2); }) ==
          "runtime error: invalid memory address or nil pointer dereference"); // r->W in Scale
    // A nil interface is another thing.
    any nothing;
    CHECK(nothing == nullptr);
    CHECK(nothing.gp_type() != x.gp_type());
    // A nil pointer of two types gives two interface values of two types, both not nil.
    any y = to_iface<any>(ptr<Circle>(), td_pCircle());
    CHECK(y != nullptr && y.gp_type() != x.gp_type());
    // A pointer that is set again afterwards doesn't change the interface (a copy of the
    // pointer is in it).
    ptr<Rect> later;
    any z = to_iface<any>(later, td_pRect());
    later = new_<Rect>();
    CHECK(unbox<ptr<Rect>>(z) == nullptr);
}

// --- copies, moves, destruction ---

void test_lifetime() {
    using namespace gp;
    static const type_desc td_Counted =
        named<Counted>("main.Counted", "main", shape_struct(nullptr, 0));
    Counted::made = Counted::copies = Counted::destroyed = 0;
    {
        any a = to_iface<any>(Counted(1), &td_Counted); // a temporary, moved into the box
        CHECK(Counted::made - Counted::destroyed == 1 && Counted::copies == 0);
        any b = a; // shares the box
        any c = b;
        CHECK(Counted::made - Counted::destroyed == 1 && a.gp_data().use_count() == 3);
        CHECK(&unbox<Counted>(a) == &unbox<Counted>(c));
        b = to_iface<any>(Counted(2), &td_Counted);
        CHECK(Counted::made - Counted::destroyed == 2 && unbox<Counted>(b).id == 2);
        a = gp::any();
        CHECK(Counted::made - Counted::destroyed == 2); // c still has the first
        c = b;
        CHECK(Counted::made - Counted::destroyed == 1);
        // Assigning an interface to itself keeps it.
        const any& self = b;
        b = self;
        CHECK(unbox<Counted>(b).id == 2 && Counted::made - Counted::destroyed == 1);
        // A copy of the value into the box (an lvalue): one copy.
        Counted v(3);
        any d = to_iface<any>(v, &td_Counted);
        CHECK(Counted::copies == 1 && unbox<Counted>(d).id == 3);
    }
    CHECK(Counted::made == Counted::destroyed);
}

// An interface that holds a struct that holds an interface: a list. Freeing the head frees
// the rest without a deep recursion (a chain of 200,000 overflows the stack at -O0 otherwise).
struct Link {
    gp::any next;
    int64_t value = 0;
};

void test_chain() {
    using namespace gp;
    static const type_desc td_Link = named<Link>("main.Link", "main", shape_struct(nullptr, 0));
    constexpr int n = 200000;
    any head;
    for (int i = 0; i < n; i++) {
        head = to_iface<any>(Link{std::move(head), i}, &td_Link);
    }
    int64_t sum = 0;
    int count = 0;
    {
        // Walk a part of it: the values are there, in order.
        any cur = head;
        while (cur != nullptr && count < 1000) {
            const Link& l = unbox<Link>(cur);
            sum += l.value;
            any next = l.next;
            cur = std::move(next);
            count++;
        }
    }
    CHECK(count == 1000 && sum == int64_t(n - 1) * 1000 - int64_t(999) * 1000 / 2);
    head = gp::any();
    CHECK(head == nullptr);
    // The same through a pointer to a struct that holds an interface.
    struct Node {
        gp::any next;
    };
    static const type_desc td_Node = named<Node>("main.Node", "main", shape_struct(nullptr, 0));
    const type_desc* td_pNode = td_ptr<Node>(&td_Node);
    any h2;
    for (int i = 0; i < n; i++) {
        ptr<Node> p = new_<Node>();
        p->next = std::move(h2);
        h2 = to_iface<any>(std::move(p), td_pNode);
    }
    h2 = any();
    CHECK(h2 == nullptr);
}

// --- itabs ---

void test_itab_building() {
    using namespace gp;
    const type_desc* prect = td_pRect();
    const std::size_t builds0 = detail::itab_builds();
    const std::size_t cached0 = detail::itab_cached();

    // *Rect has Area, Name and Scale: it implements Shape, Areaer and Namer.
    const itab* t1 = getitab(&gp_td_Shape, prect);
    CHECK(t1 != nullptr && t1->gp_inter == &gp_td_Shape && t1->gp_type == prect);
    CHECK(t1->gp_missing == nullptr && t1->gp_fun != nullptr);
    // The slots are the thunks, in the interface's order: Area, Scale.
    CHECK(t1->gp_fun[0] == reinterpret_cast<fn_ptr>(&thunk_pRect_Area));
    CHECK(t1->gp_fun[1] == reinterpret_cast<fn_ptr>(&thunk_pRect_Scale));
    CHECK(detail::itab_builds() == builds0 + 1 && detail::itab_cached() == cached0 + 1);
    // A hit: the same itab, nothing built.
    CHECK(getitab(&gp_td_Shape, prect) == t1);
    CHECK(getitab(&gp_td_Shape, prect) == t1);
    CHECK(detail::itab_builds() == builds0 + 1);
    CHECK(implements(&gp_td_Shape, prect));
    CHECK(missing_method(&gp_td_Shape, prect) == nullptr);
    CHECK(detail::itab_builds() == builds0 + 1);

    // Another interface: another itab, with the slot of the method it asks for.
    const itab* t2 = getitab(&gp_td_Namer, prect);
    CHECK(t2 != nullptr && t2 != t1 &&
          t2->gp_fun[0] == reinterpret_cast<fn_ptr>(&thunk_pRect_Name));
    const itab* t3 = getitab(&gp_td_Areaer, prect);
    CHECK(t3 != nullptr && t3->gp_fun[0] == reinterpret_cast<fn_ptr>(&thunk_pRect_Area));
    CHECK(detail::itab_builds() == builds0 + 3);

    // Rect (the value) has only the value methods: Area and Name, but not Scale.
    CHECK(getitab(&gp_td_Areaer, &gp_td_Rect) != nullptr);
    CHECK(getitab(&gp_td_Areaer, &gp_td_Rect)->gp_fun[0] ==
          reinterpret_cast<fn_ptr>(&thunk_Rect_Area));
    CHECK(getitab(&gp_td_Namer, &gp_td_Rect) != nullptr);
    CHECK(getitab(&gp_td_Areaer, &gp_td_Rect) !=
          getitab(&gp_td_Areaer, prect)); // two types, two itabs
    const std::size_t builds1 = detail::itab_builds();
    CHECK(getitab(&gp_td_Shape, &gp_td_Rect) == nullptr);
    const imethod* missing = missing_method(&gp_td_Shape, &gp_td_Rect);
    CHECK(missing != nullptr && std::string(missing->gp_name) == "Scale");
    CHECK(!implements(&gp_td_Shape, &gp_td_Rect));
    // A miss is cached too: asking again builds nothing.
    CHECK(detail::itab_builds() == builds1 + 1);
    // The entry of a miss: it names the missing method and has no slots (so it can't be called).
    const itab* neg = detail::itab_entry(&gp_td_Shape, &gp_td_Rect);
    CHECK(neg != nullptr && neg->gp_missing == missing && neg->gp_fun == nullptr);
    CHECK(neg->gp_inter == &gp_td_Shape && neg->gp_type == &gp_td_Rect);
    // The entry of a hit is the itab that getitab gives.
    CHECK(detail::itab_entry(&gp_td_Shape, prect) == t1 && t1->gp_missing == nullptr);
    CHECK(detail::itab_builds() == builds1 + 1);
    CHECK(getitab(&gp_td_Shape, &gp_td_Rect) == nullptr);
    CHECK(getitab(&gp_td_Shape, &gp_td_Rect) == nullptr);
    CHECK(missing_method(&gp_td_Shape, &gp_td_Rect) == missing);
    CHECK(detail::itab_builds() == builds1 + 1);

    // Circle has no value methods; *Circle has Area only.
    CHECK(getitab(&gp_td_Areaer, &gp_td_Circle) == nullptr);
    CHECK(std::string(missing_method(&gp_td_Areaer, &gp_td_Circle)->gp_name) == "Area");
    CHECK(getitab(&gp_td_Areaer, td_pCircle()) != nullptr);
    CHECK(getitab(&gp_td_Shape, td_pCircle()) == nullptr);
    CHECK(std::string(missing_method(&gp_td_Shape, td_pCircle())->gp_name) == "Scale");
    // A type without any method.
    CHECK(getitab(&gp_td_Areaer, &td_int) == nullptr);
    CHECK(getitab(&gp_td_Areaer, td_slice<int64_t>(&td_int)) == nullptr);
    CHECK(getitab(&gp_td_Areaer, foreign_td<int>()) == nullptr);

    // The empty interface: every type implements it, and it needs no itab.
    CHECK(getitab(&td_any, &td_int) == nullptr);
    CHECK(implements(&td_any, &td_int) && implements(&td_any, prect));
    CHECK(missing_method(&td_any, &td_int) == nullptr);
    CHECK(implements(td_interface({}), &gp_td_Rect));
}

// A type matches an interface by name, package and the TEXT of the signature.
void test_itab_signatures() {
    using namespace gp;
    static const method ok_methods[] = {
        method("Get", nullptr, "func() C.int", reinterpret_cast<fn_ptr>(&thunk_Rect_Area))};
    static const type_desc td_gets_int =
        named<Rect>("main.GetsInt", "main", shape_struct(nullptr, 0), methods_of(ok_methods));
    static const method dbl_methods[] = {
        method("Get", nullptr, "func() C.double", reinterpret_cast<fn_ptr>(&thunk_Rect_Area))};
    static const type_desc td_gets_double =
        named<Rect>("main.GetsDouble", "main", shape_struct(nullptr, 0), methods_of(dbl_methods));
    const type_desc* iface_int = td_interface({imethod("Get", nullptr, "func() C.int")});
    const type_desc* iface_double = td_interface({imethod("Get", nullptr, "func() C.double")});
    // go/types would accept all four (every C type is invalid for it, and two invalid types are
    // equal); the signature texts tell them apart.
    CHECK(getitab(iface_int, &td_gets_int) != nullptr);
    CHECK(getitab(iface_double, &td_gets_double) != nullptr);
    CHECK(getitab(iface_int, &td_gets_double) == nullptr);
    CHECK(getitab(iface_double, &td_gets_int) == nullptr);
    CHECK(std::string(missing_method(iface_int, &td_gets_double)->gp_name) == "Get");
    // Plain Go signatures: one parameter more or a different result is another method.
    static const method sig_methods[] = {
        method("Do", nullptr, "func(int) error", reinterpret_cast<fn_ptr>(&thunk_Rect_Area))};
    static const type_desc td_do =
        named<Rect>("main.Do", "main", shape_struct(nullptr, 0), methods_of(sig_methods));
    CHECK(getitab(td_interface({imethod("Do", nullptr, "func(int) error")}), &td_do) != nullptr);
    CHECK(getitab(td_interface({imethod("Do", nullptr, "func(int64) error")}), &td_do) == nullptr);
    CHECK(getitab(td_interface({imethod("Do", nullptr, "func(int, int) error")}), &td_do) ==
          nullptr);
    CHECK(getitab(td_interface({imethod("Do", nullptr, "func(int)")}), &td_do) == nullptr);
    CHECK(getitab(td_interface({imethod("Do", nullptr, "func(int) error"),
                                imethod("Undo", nullptr, "func()")}),
                  &td_do) == nullptr);
    // The name matters, in the way that the sorted walk could get wrong: methods before and
    // after the one the interface wants.
    static const method many[] = {
        method("A", nullptr, "func()", reinterpret_cast<fn_ptr>(&thunk_Rect_Area)),
        method("C", nullptr, "func()", reinterpret_cast<fn_ptr>(&thunk_Rect_Area)),
        method("E", nullptr, "func()", reinterpret_cast<fn_ptr>(&thunk_Rect_Area)),
        method("a", "p", "func()", reinterpret_cast<fn_ptr>(&thunk_Rect_Area))};
    static const type_desc td_many =
        named<Rect>("main.Many", "main", shape_struct(nullptr, 0), methods_of(many));
    CHECK(getitab(td_interface({imethod("C", nullptr, "func()")}), &td_many) != nullptr);
    CHECK(getitab(td_interface({imethod("A", nullptr, "func()"), imethod("E", nullptr, "func()")}),
                  &td_many) != nullptr);
    CHECK(getitab(td_interface({imethod("A", nullptr, "func()"), imethod("C", nullptr, "func()"),
                                imethod("E", nullptr, "func()"), imethod("a", "p", "func()")}),
                  &td_many) != nullptr);
    CHECK(getitab(td_interface({imethod("B", nullptr, "func()")}), &td_many) == nullptr);
    CHECK(getitab(td_interface({imethod("F", nullptr, "func()")}), &td_many) == nullptr);
    CHECK(getitab(td_interface({imethod("A", nullptr, "func()"), imethod("B", nullptr, "func()")}),
                  &td_many) == nullptr);
    CHECK(std::string(missing_method(td_interface({imethod("A", nullptr, "func()"),
                                                   imethod("B", nullptr, "func()"),
                                                   imethod("E", nullptr, "func()")}),
                                     &td_many)
                          ->gp_name) == "B");
}

// An unexported method belongs to its package: another package's m() never satisfies the
// interface.
void test_itab_unexported() {
    using namespace gp;
    static const method in_a[] = {
        method("m", "example.com/a", "func()", reinterpret_cast<fn_ptr>(&thunk_Rect_Area))};
    static const method in_b[] = {
        method("m", "example.com/b", "func()", reinterpret_cast<fn_ptr>(&thunk_Rect_Area))};
    static const method both[] = {
        method("m", "example.com/a", "func()", reinterpret_cast<fn_ptr>(&thunk_Rect_Area)),
        method("m", "example.com/b", "func()", reinterpret_cast<fn_ptr>(&thunk_Rect_Area))};
    static const type_desc td_a =
        named<Rect>("a.T", "example.com/a", shape_struct(nullptr, 0), methods_of(in_a));
    static const type_desc td_b =
        named<Rect>("b.T", "example.com/b", shape_struct(nullptr, 0), methods_of(in_b));
    static const type_desc td_ab =
        named<Rect>("c.T", "example.com/c", shape_struct(nullptr, 0), methods_of(both));
    const type_desc* want_a = td_interface({imethod("m", "example.com/a", "func()")});
    const type_desc* want_b = td_interface({imethod("m", "example.com/b", "func()")});
    const type_desc* exported = td_interface({imethod("M", nullptr, "func()")});
    CHECK(getitab(want_a, &td_a) != nullptr && getitab(want_b, &td_b) != nullptr);
    CHECK(getitab(want_a, &td_b) == nullptr && getitab(want_b, &td_a) == nullptr);
    CHECK(getitab(want_a, &td_ab) != nullptr && getitab(want_b, &td_ab) != nullptr);
    // An exported M and an unexported m are different methods, too.
    CHECK(getitab(exported, &td_a) == nullptr);
    // Both packages' methods: an interface with both is satisfied by the type with both only.
    const type_desc* want_ab = td_interface(
        {imethod("m", "example.com/a", "func()"), imethod("m", "example.com/b", "func()")});
    CHECK(getitab(want_ab, &td_ab) != nullptr && getitab(want_ab, &td_a) == nullptr);
    CHECK(getitab(want_ab, &td_ab)->gp_fun[1] == reinterpret_cast<fn_ptr>(&thunk_Rect_Area));
}

// --- conversions and calls ---

void test_to_iface() {
    using namespace gp;
    const std::size_t builds0 = detail::itab_builds();
    Shape s = to_iface<Shape>(td_pRect() == nullptr ? gp::ptr<Rect>() : new_<Rect>(Rect{2, 3}),
                              td_pRect());
    CHECK(s != nullptr && s.gp_type() == td_pRect());
    CHECK(s.gp_tab() == getitab(&gp_td_Shape, td_pRect()));
    CHECK(s.gp_tab()->gp_inter == &gp_td_Shape);
    // The method calls go through the itab.
    CHECK(s.Area() == 6);
    s.Scale(2);
    CHECK(s.Area() == 24);
    CHECK(unbox<ptr<Rect>>(s)->W == 4);
    // An interface value that holds a Rect (the value) for an interface that Rect satisfies.
    Areaer a = to_iface<Areaer>(Rect{3, 4}, &gp_td_Rect);
    CHECK(a.Area() == 12);
    CHECK(a.gp_tab() == getitab(&gp_td_Areaer, &gp_td_Rect));
    Namer n = to_iface<Namer>(Rect{}, &gp_td_Rect);
    CHECK(n.Name() == "rect"_gs);
    // Each (interface, C++ type) pair looks its itab up once: four conversions of the same pair
    // build nothing more.
    const std::size_t builds1 = detail::itab_builds();
    for (int i = 0; i < 4; i++) {
        Areaer again = to_iface<Areaer>(Rect{double(i), 2}, &gp_td_Rect);
        CHECK(again.Area() == 2 * i);
    }
    CHECK(detail::itab_builds() == builds1);
    (void)builds0;
    // The empty interface: no itab, whatever the type.
    CHECK(to_iface<any>(Rect{}, &gp_td_Rect).gp_tab() == nullptr);
    // Every interface value converts to any: the same box and type, without the itab.
    any boxed = s;
    CHECK(boxed.gp_type() == s.gp_type() && boxed.gp_data() == s.gp_data() &&
          boxed.gp_tab() == nullptr);
    any from_a = a;
    CHECK(from_a.gp_type() == &gp_td_Rect && unbox<Rect>(from_a).W == 3);
    // Copying an any keeps what it holds.
    any again = from_a;
    CHECK(again.gp_type() == &gp_td_Rect && again.gp_data() == from_a.gp_data());
}

// Two Go types can share one C++ type: the cached itab of a conversion site is checked
// against the descriptor, so the second Go type gets its own methods.
int64_t g_calls = 0;
int64_t thunk_num_a(const gp::iface&) {
    g_calls += 1;
    return 1;
}
int64_t thunk_num_b(const gp::iface&) {
    g_calls += 100;
    return 2;
}

struct Numberer : gp::iface {
    using gp::iface::iface;
    static const gp::type_desc* gp_desc();
    int64_t Num() const { return gp::icall<int64_t()>(*this, 0); }
};
const gp::imethod gp_im_Numberer[] = {gp::imethod("Num", nullptr, "func() int")};
const gp::type_desc gp_td_Numberer =
    gp::named<Numberer>("main.Numberer", "main", gp::shape_interface(gp_im_Numberer));
const gp::type_desc* Numberer::gp_desc() {
    return &gp_td_Numberer;
}

void test_site_cache() {
    using namespace gp;
    // type A int64 and type B int64: one C++ type (int64_t here), two Go types, each with a Num.
    static const method ma[] = {method_of<int64_t()>("Num", nullptr, "func() int", &thunk_num_a)};
    static const method mb[] = {method_of<int64_t()>("Num", nullptr, "func() int", &thunk_num_b)};
    static const type_desc td_A =
        named<int64_t>("main.A", "main", shape_basic(kind::Int64), methods_of(ma));
    static const type_desc td_B =
        named<int64_t>("main.B", "main", shape_basic(kind::Int64), methods_of(mb));
    g_calls = 0;
    for (int round = 0; round < 3; round++) {
        Numberer x = to_iface<Numberer>(int64_t(1), &td_A);
        Numberer y = to_iface<Numberer>(int64_t(1), &td_B);
        CHECK(x.Num() == 1 && y.Num() == 2);
        CHECK(x.gp_tab() != y.gp_tab() && x.gp_type() == &td_A && y.gp_type() == &td_B);
    }
    CHECK(g_calls == 3 * 101);
}

// Arguments and results of every shape go through the thunk.
struct Mixer {
    int64_t base = 0;
};

struct Calc : gp::iface {
    using gp::iface::iface;
    static const gp::type_desc* gp_desc();
    gp::string Mix(int64_t n, gp::string s, gp::slice<int64_t> v, double d) const {
        return gp::icall<gp::string(int64_t, gp::string, gp::slice<int64_t>, double)>(*this, 1, n,
                                                                                      s, v, d);
    }
    std::tuple<int64_t, bool> Split(int64_t x) const {
        return gp::icall<std::tuple<int64_t, bool>(int64_t)>(*this, 2, x);
    }
    void Bump() const { gp::icall<void()>(*this, 0); }
    // The same call, for an interface that the method could change.
    gp::string MixKeep(int64_t n) const {
        return gp::icall_keep<gp::string(int64_t, gp::string, gp::slice<int64_t>, double)>(
            *this, 1, n, gp::string(), gp::slice<int64_t>(), 0.0);
    }
};

int64_t g_bumps = 0;
void thunk_Mixer_Bump(const gp::iface& self) {
    g_bumps += gp::unbox<Mixer>(self).base;
}
gp::string thunk_Mixer_Mix(const gp::iface& self, int64_t n, gp::string s, gp::slice<int64_t> v,
                           double d) {
    return s + gp::string::gp_copy(
                   std::to_string(gp::unbox<Mixer>(self).base + n + gp::len(v) + int64_t(d)));
}
std::tuple<int64_t, bool> thunk_Mixer_Split(const gp::iface& self, int64_t x) {
    return {x / gp::unbox<Mixer>(self).base, x % 2 == 1};
}

const gp::method gp_vm_Mixer[] = {
    gp::method_of<void()>("Bump", nullptr, "func()", &thunk_Mixer_Bump),
    gp::method_of<gp::string(int64_t, gp::string, gp::slice<int64_t>, double)>(
        "Mix", nullptr, "func(int, string, []int, float64) string", &thunk_Mixer_Mix),
    gp::method_of<std::tuple<int64_t, bool>(int64_t)>("Split", nullptr, "func(int) (int, bool)",
                                                      &thunk_Mixer_Split),
};
const gp::imethod gp_im_Calc[] = {
    gp::imethod("Bump", nullptr, "func()"),
    gp::imethod("Mix", nullptr, "func(int, string, []int, float64) string"),
    gp::imethod("Split", nullptr, "func(int) (int, bool)"),
};
const gp::type_desc gp_td_Calc =
    gp::named<Calc>("main.Calc", "main", gp::shape_interface(gp_im_Calc));
const gp::type_desc gp_td_Mixer = gp::named<Mixer>(
    "main.Mixer", "main", gp::shape_struct(nullptr, 0), gp::methods_of(gp_vm_Mixer));
const gp::type_desc* Calc::gp_desc() {
    return &gp_td_Calc;
}

void test_icall() {
    using namespace gp;
    Calc c = to_iface<Calc>(Mixer{10}, &gp_td_Mixer);
    CHECK(c.Mix(5, "n="_gs, slice_of<int64_t>({1, 2, 3}), 2.9) == "n=20"_gs);
    auto [q, odd] = c.Split(25);
    CHECK(q == 2 && odd);
    g_bumps = 0;
    c.Bump();
    c.Bump();
    CHECK(g_bumps == 20);
    // The arguments are passed by value: a string and a slice that the caller still has.
    gp::string s = gp::string::gp_copy(std::string(50, 'z'));
    slice<int64_t> v = slice_of<int64_t>({4, 5});
    CHECK(c.Mix(0, s, v, 0) == s + "12"_gs);
    CHECK(gp::len(s) == 50 && gp::len(v) == 2);
    // A call through a copy of the interface value.
    Calc c2 = c;
    CHECK(c2.Mix(1, ""_gs, slice<int64_t>(), 1) == "12"_gs);
    CHECK(c.MixKeep(7) == "17"_gs);
    CHECK(c.gp_data().use_count() == 2); // the keep copy is gone
}

// icall_keep holds the box alive for the call: the method here drops the last interface value
// that holds it, as `s.f.M()` does when M assigns s.f.
Shape g_holder;
double thunk_selfdrop_Area(const gp::iface& self) {
    const Rect& r = gp::unbox<Rect>(self);
    g_holder = Shape(); // the box is freed here, unless the call holds it
    return r.W + r.H;   // read after: a use after free when the box is gone (ASan finds it)
}

void test_keep() {
    using namespace gp;
    static const method sm[] = {
        method_of<double()>("Area", nullptr, "func() float64", &thunk_selfdrop_Area),
        method_of<void(double)>("Scale", nullptr, "func(float64)", &thunk_pRect_Scale)};
    static const type_desc td_selfdrop =
        named<Rect>("main.SelfDrop", "main", shape_struct(nullptr, 0), methods_of(sm));
    g_holder = to_iface<Shape>(Rect{3, 4}, &td_selfdrop);
    CHECK(g_holder.gp_data().use_count() == 1);
    double got = gp::icall_keep<double()>(g_holder, 0);
    CHECK(got == 7);
    CHECK(g_holder == nullptr);
}

// --- C and C++ values in an interface ---

struct CxxPoint {
    int x = 0;
    int y = 0;
    bool operator==(const CxxPoint&) const = default;
};

void test_foreign_values() {
    using namespace gp;
    any s = to_iface<any>(std::string("hello"), foreign_td<std::string>());
    CHECK(unbox<std::string>(s) == "hello" && s.gp_type() == foreign_td<std::string>());
    CHECK(s.gp_type()->gp_kind == kind::Foreign);
    any p = to_iface<any>(CxxPoint{1, 2}, foreign_td<CxxPoint>());
    CHECK(unbox<CxxPoint>(p).y == 2);
    // A C int in an interface is not a Go int32 in one: the descriptors tell them apart.
    any c = to_iface<any>(int(7), foreign_td<int>());
    any g = to_iface<any>(int32_t(7), &td_int32);
    CHECK(c.gp_type() != g.gp_type() && c.gp_type() != &td_int);
    CHECK(unbox<int>(c) == 7 && unbox<int32_t>(g) == 7);
    // A raw pointer is boxed as a value (it has no owner).
    int v = 5;
    any rp = to_iface<any>(&v, foreign_td<int*>());
    CHECK(unbox<int*>(rp) == &v && rp.gp_type() == foreign_td<int*>());
    // The method sets of foreign types are empty.
    CHECK(getitab(&gp_td_Areaer, foreign_td<std::string>()) == nullptr);
    CHECK(implements(&td_any, foreign_td<std::string>()));
    // A std::vector in an interface is freed with it (ASan and LeakSanitizer look).
    any vec = to_iface<any>(std::vector<int>(1000, 1), foreign_td<std::vector<int>>());
    CHECK(unbox<std::vector<int>>(vec).size() == 1000);
}

// --- many threads ---

// make_types makes n_types types and n_ifaces interfaces over methods M0..M4: type k has the
// methods whose bit is set in k, interface j those whose bit is set in j (the bit of M0 first).
// A type implements an interface when it has every method of it.
struct world {
    static constexpr int methods_n = 5;
    static constexpr int count = 1 << methods_n;
    std::deque<std::string> names;
    std::deque<std::vector<gp::method>> tables;
    std::deque<std::vector<gp::imethod>> itables;
    std::vector<const gp::type_desc*> types;
    std::vector<const gp::type_desc*> ifaces;
    std::deque<gp::type_desc> store;
};

void dummy_thunk(const gp::iface&) {}

world make_world() {
    static const char* const method_names[] = {"M0", "M1", "M2", "M3", "M4"};
    world w;
    for (int mask = 0; mask < world::count; mask++) {
        w.tables.emplace_back();
        w.itables.emplace_back();
        for (int b = 0; b < world::methods_n; b++) {
            if (mask & (1 << b)) {
                w.tables.back().push_back(
                    gp::method_of<void()>(method_names[b], nullptr, "func()", &dummy_thunk));
                w.itables.back().push_back(gp::imethod(method_names[b], nullptr, "func()"));
            }
        }
        w.names.push_back("T" + std::to_string(mask));
        w.store.push_back(
            gp::named<Rect>(w.names.back().c_str(), "main", gp::shape_struct(nullptr, 0),
                            gp::method_table{w.tables.back().data(), w.tables.back().size()}));
        w.types.push_back(&w.store.back());
        w.names.push_back("I" + std::to_string(mask));
        w.store.push_back(gp::named<Shape>(
            w.names.back().c_str(), "main",
            gp::shape_interface(w.itables.back().data(), w.itables.back().size())));
        w.ifaces.push_back(&w.store.back());
    }
    return w;
}

void test_threads() {
    using namespace gp;
    world w = make_world();
    constexpr int threads_n = 8;
    constexpr int pairs = world::count * world::count;
    const std::size_t builds0 = detail::itab_builds();
    std::vector<std::vector<const itab*>> got(threads_n, std::vector<const itab*>(pairs));
    std::vector<std::thread> threads;
    for (int t = 0; t < threads_n; t++) {
        threads.emplace_back([&, t] {
            for (int k = 0; k < pairs; k++) {
                const int i = (k * 37 + t * 101) % pairs; // each thread in its own order
                const int ifc = i / world::count;
                const int typ = i % world::count;
                if (ifc == 0) {
                    got[t][i] = nullptr; // the empty interface needs no itab
                    continue;
                }
                got[t][i] = getitab(w.ifaces[ifc], w.types[typ]);
            }
        });
    }
    for (auto& t : threads) {
        t.join();
    }
    int built = 0;
    for (int ifc = 0; ifc < world::count; ifc++) {
        for (int typ = 0; typ < world::count; typ++) {
            const int i = ifc * world::count + typ;
            const bool want = (ifc & ~typ) == 0; // the type has every method of the interface
            if (ifc == 0) {
                CHECK(got[0][i] == nullptr);
                continue;
            }
            built++;
            for (int t = 0; t < threads_n; t++) {
                CHECK(got[t][i] == got[0][i]);
            }
            CHECK((got[0][i] != nullptr) == want);
            if (want) {
                // the slots are the type's thunks, in the interface's order
                CHECK(got[0][i]->gp_inter == w.ifaces[ifc] && got[0][i]->gp_type == w.types[typ]);
                CHECK(got[0][i]->gp_fun[0] == reinterpret_cast<fn_ptr>(&dummy_thunk));
            }
        }
    }
    // Each pair was built once, however many threads asked at once.
    CHECK(detail::itab_builds() == builds0 + static_cast<std::size_t>(built));

    // Conversions from many threads at once, through the per-site caches and the global one.
    std::vector<std::thread> workers;
    int64_t sums[threads_n] = {};
    for (int t = 0; t < threads_n; t++) {
        workers.emplace_back([&sums, t] {
            for (int i = 0; i < 2000; i++) {
                Areaer a = to_iface<Areaer>(Rect{double(i), 2}, &gp_td_Rect);
                Shape s = to_iface<Shape>(new_<Rect>(Rect{1, double(i)}), td_pRect());
                sums[t] += int64_t(a.Area() + s.Area());
            }
        });
    }
    for (auto& t : workers) {
        t.join();
    }
    for (int t = 1; t < threads_n; t++) {
        CHECK(sums[t] == sums[0]);
    }
    CHECK(sums[0] == int64_t(2000) * 1999 / 2 * 2 + int64_t(2000) * 1999 / 2);
}

// --- the hooks of the descriptors, through the data of an interface ---

// PS is a struct with a pointer, comparable in Go.
struct PS {
    gp::ptr<Rect> p;
    gp::string s;
    bool operator==(const PS&) const = default;
    std::size_t gp_hash() const { return gp::hash_fields(p, s); }
};

// same_type_equal compares two interface values of one dynamic type with the descriptor's
// hook, on their data, as interface == will (step 5.6); the hash the same.
bool hooks_equal(const gp::any& a, const gp::any& b) {
    return a.gp_type()->gp_equal != nullptr &&
           a.gp_type()->gp_equal(a.gp_data().get(), b.gp_data().get());
}
std::size_t hooks_hash(const gp::any& a) {
    return a.gp_type()->gp_hash(a.gp_data().get());
}

void test_hooks_through_iface() {
    using namespace gp;
    // Pointers: the data of the interface is the target address, so that is what the hooks get.
    ptr<Rect> p1 = new_<Rect>(Rect{1, 2});
    ptr<Rect> p2 = new_<Rect>(Rect{1, 2}); // equal contents, another object
    const type_desc* tp = td_pRect();
    any a1 = to_iface<any>(p1, tp), a1b = to_iface<any>(p1, tp), a2 = to_iface<any>(p2, tp);
    CHECK(hooks_equal(a1, a1b) && !hooks_equal(a1, a2));
    CHECK(hooks_hash(a1) == hooks_hash(a1b));
    CHECK(hooks_hash(a1) == gp::hash_of(p1)); // the hash a pointer has as a map key
    // Two nil pointers of one type are equal, and not equal to a pointer.
    any n1 = to_iface<any>(ptr<Rect>(), tp), n2 = to_iface<any>(ptr<Rect>(), tp);
    CHECK(hooks_equal(n1, n2) && !hooks_equal(n1, a1) && hooks_hash(n1) == hooks_hash(n2));
    // An interior pointer: equal to another pointer to the same field, not to the object.
    ptr<double> w1(p1, &p1->H), w2(p1, &p1->H), w0(p1, &p1->W);
    const type_desc* tw = td_ptr<double>(&td_float64);
    any iw1 = to_iface<any>(w1, tw), iw2 = to_iface<any>(w2, tw), iw0 = to_iface<any>(w0, tw);
    CHECK(hooks_equal(iw1, iw2) && !hooks_equal(iw1, iw0));
    // unsafe.Pointer.
    int x = 0, y = 0;
    ptr<void> u1(static_owner(), &x), u2(static_owner(), &x), u3(static_owner(), &y);
    any iu1 = to_iface<any>(u1, &td_unsafe_pointer), iu2 = to_iface<any>(u2, &td_unsafe_pointer),
        iu3 = to_iface<any>(u3, &td_unsafe_pointer);
    CHECK(hooks_equal(iu1, iu2) && !hooks_equal(iu1, iu3) && hooks_hash(iu1) == hooks_hash(iu2));
    any inil = to_iface<any>(ptr<void>(), &td_unsafe_pointer);
    CHECK(hooks_equal(inil, to_iface<any>(ptr<void>(), &td_unsafe_pointer)) &&
          !hooks_equal(inil, iu1));

    // Boxed values: the data is the box, an object of the C++ type.
    CHECK(hooks_equal(to_iface<any>(int64_t(5), &td_int), to_iface<any>(int64_t(5), &td_int)));
    CHECK(!hooks_equal(to_iface<any>(int64_t(5), &td_int), to_iface<any>(int64_t(6), &td_int)));
    CHECK(hooks_hash(to_iface<any>(int64_t(5), &td_int)) ==
          hooks_hash(to_iface<any>(int64_t(5), &td_int)));
    gp::string long1 = gp::string::gp_copy(std::string(60, 'q')),
               long2 = gp::string::gp_copy(std::string(60, 'q'));
    CHECK(hooks_equal(to_iface<any>(long1, &td_string), to_iface<any>(long2, &td_string)));
    CHECK(hooks_hash(to_iface<any>(long1, &td_string)) ==
          hooks_hash(to_iface<any>(long2, &td_string)));
    CHECK(!hooks_equal(to_iface<any>("a"_gs, &td_string), to_iface<any>("b"_gs, &td_string)));
    double nan = std::nan("");
    CHECK(!hooks_equal(to_iface<any>(nan, &td_float64), to_iface<any>(nan, &td_float64)));
    CHECK(hooks_equal(to_iface<any>(0.0, &td_float64), to_iface<any>(-0.0, &td_float64)));
    CHECK(hooks_equal(to_iface<any>(Rect{1, 2}, &gp_td_Rect),
                      to_iface<any>(Rect{1, 2}, &gp_td_Rect)));
    CHECK(!hooks_equal(to_iface<any>(Rect{1, 2}, &gp_td_Rect),
                       to_iface<any>(Rect{1, 3}, &gp_td_Rect)));
    // A struct with a pointer in it: the struct's own ==, the pointer by address.
    static const type_desc td_PS = named<PS>("main.PS", "main", shape_struct(nullptr, 0));
    CHECK(td_PS.gp_equal != nullptr && td_PS.gp_hash != nullptr);
    any s1 = to_iface<any>(PS{p1, "x"_gs}, &td_PS), s1b = to_iface<any>(PS{p1, "x"_gs}, &td_PS),
        s2 = to_iface<any>(PS{p2, "x"_gs}, &td_PS);
    CHECK(hooks_equal(s1, s1b) && !hooks_equal(s1, s2) && hooks_hash(s1) == hooks_hash(s1b));
    // An array of strings.
    const type_desc* tas = td_array<gp::string, 2>(&td_string);
    gp::array<gp::string, 2> as1{"a"_gs, "b"_gs}, as2{"a"_gs, "b"_gs}, as3{"a"_gs, "c"_gs};
    any ia1 = to_iface<any>(as1, tas), ia2 = to_iface<any>(as2, tas), ia3 = to_iface<any>(as3, tas);
    CHECK(hooks_equal(ia1, ia2) && !hooks_equal(ia1, ia3) && hooks_hash(ia1) == hooks_hash(ia2));
    // Not comparable in Go: no hook, so interface == (step 5.6) can panic with "comparing
    // uncomparable type []int", also for an array of slices and a func and a map.
    any sl = to_iface<any>(slice_of<int64_t>({1}), td_slice<int64_t>(&td_int));
    CHECK(sl.gp_type()->gp_equal == nullptr && sl.gp_type()->gp_hash == nullptr);
    any asl = to_iface<any>(gp::array<gp::slice<int64_t>, 1>{},
                            td_array<gp::slice<int64_t>, 1>(sl.gp_type()));
    CHECK(asl.gp_type()->gp_equal == nullptr);
    any mp = to_iface<any>(gp::map<gp::string, int64_t>(),
                           td_map<gp::string, int64_t>(&td_string, &td_int));
    CHECK(mp.gp_type()->gp_equal == nullptr);
    any fn = to_iface<any>(gp::func<void()>(), td_func<gp::func<void()>>({}, {}));
    CHECK(fn.gp_type()->gp_equal == nullptr);
}

// --- the cases that end the program: a fatal error of the runtime (cxx_test.go runIface) ---

const gp::method gp_unsorted[] = {
    gp::method_of<double()>("Name", nullptr, "func() float64", &thunk_Rect_Area),
    gp::method_of<double()>("Area", nullptr, "func() float64", &thunk_Rect_Area),
};

int run_case(const std::string& c) {
    using namespace gp;
    if (c == "unsorted-type") {
        static const type_desc td =
            named<Rect>("main.Unsorted", "main", shape_struct(nullptr, 0), methods_of(gp_unsorted));
        (void)getitab(&gp_td_Areaer, &td);
    } else if (c == "unsorted-iface") {
        (void)td_interface({imethod("B", nullptr, "func()"), imethod("A", nullptr, "func()")});
    } else if (c == "unsorted-iface-desc") {
        static const imethod bad[] = {imethod("B", nullptr, "func()"),
                                      imethod("A", nullptr, "func()")};
        static const type_desc td = named<Shape>("main.BadIface", "main", shape_interface(bad));
        (void)getitab(&td, &gp_td_Rect);
    } else if (c == "bad-conversion") {
        (void)to_iface<Shape>(Rect{1, 2}, &gp_td_Rect); // Rect has no Scale
    } else if (c == "bad-unbox") {
        (void)unbox<Rect>(any());
    } else if (c == "interface-name") {
        (void)td_interface({imethod("Z1", nullptr, "func()")}, "interface { Z1() }");
        (void)td_interface({imethod("Z1", nullptr, "func()")}, "interface { Z1(); }");
    } else if (c == "getitab-concrete") {
        (void)getitab(&gp_td_Areaer, &gp_td_Shape); // an interface type is not a concrete type
    } else {
        return 3;
    }
    return 0;
}

void run_tests() {
    test_nil();
    test_itab_building(); // first: it counts the itabs that it builds
    test_small_values();
    test_large_values();
    test_pointers();
    test_nil_pointer_in_interface();
    test_lifetime();
    test_chain();
    test_itab_signatures();
    test_itab_unexported();
    test_to_iface();
    test_site_cache();
    test_icall();
    test_keep();
    test_foreign_values();
    test_hooks_through_iface();
    test_threads();
    if (failures > 0) {
        std::string msg =
            "FAIL: " + std::to_string(failures) + " of " + std::to_string(checks) + " checks\n";
        gp::os::write(gp::os::stderr_fd, msg.data(), msg.size());
        gp::os::exit(1);
    }
    gp::fmtshim::Printf("ok: %d checks\n"_gs, int64_t(checks));
}

void no_init() {}

} // namespace

int main(int argc, char** argv) {
    if (argc == 2) {
        // A case that ends the program (cxx_test.go runIface runs each with its name).
        static std::string name;
        name = argv[1];
        return gp::run_main(argc, argv, &no_init, [] { gp::os::exit(run_case(name)); });
    }
    return gp::run_main(argc, argv, &no_init, &run_tests);
}
