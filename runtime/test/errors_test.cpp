// The unit tests of the native functions of package errors (gocpp/errors.hpp, Phase 05 step
// 5.7): comparable, check_target and assign, over types written the way the emitter writes them.
// runtime/cxx_test.go compiles this file with the runtime's .cpp files, with g++ and clang++, at
// -O0, at -O2 and with ASan + UBSan, and runs it like test/unit.cpp. It prints "ok: N checks"
// and exits 0, or prints each failed check and exits 1.
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
//     type Coder interface{ Code() int }
//     type PtrErr struct{ msg string }           (e *PtrErr) Error() string
//     type ValErr struct{ code int }             (e ValErr) Error() string, (e ValErr) Code() int
//     type SliceErr struct{ parts []string }     (e SliceErr) Error() string
//     type NotErr struct{ n int }                no methods

struct PtrErr {
    gp::string msg;
    static gp::string Error(gp::ptr<PtrErr> e) { return e->msg; }
};

struct ValErr {
    int64_t code = 0;
    bool operator==(const ValErr&) const = default;
    gp::string Error() const { return "valerr"_gs; }
    int64_t Code() const { return code; }
};

struct SliceErr {
    gp::slice<gp::string> parts;
    gp::string Error() const { return "sliceerr"_gs; }
};

struct NotErr {
    int64_t n = 0;
};

// An error type that can be copied into an interface but not assigned (a C++ type, as a Go struct
// with a move-only C++ field would be): its descriptor has no store hook.
struct NoAssign {
    gp::string text;
    NoAssign() = default;
    explicit NoAssign(gp::string t) : text(std::move(t)) {}
    NoAssign(const NoAssign&) = default;
    NoAssign& operator=(const NoAssign&) = delete;
    gp::string Error() const { return text; }
};

const gp::type_desc* coder_desc();
struct Coder : gp::iface {
    using gp::iface::iface;
    static const gp::type_desc* gp_desc() { return coder_desc(); }
    int64_t Code() const { return gp::icall<int64_t()>(*this, 0); }
};

gp::string thunk_pPtrErr_Error(const gp::iface& self) {
    return PtrErr::Error(gp::unbox<gp::ptr<PtrErr>>(self));
}
gp::string thunk_ValErr_Error(const gp::iface& self) {
    return gp::unbox<ValErr>(self).Error();
}
int64_t thunk_ValErr_Code(const gp::iface& self) {
    return gp::unbox<ValErr>(self).Code();
}
gp::string thunk_pValErr_Error(const gp::iface& self) {
    return gp::unbox<gp::ptr<ValErr>>(self)->Error();
}
int64_t thunk_pValErr_Code(const gp::iface& self) {
    return gp::unbox<gp::ptr<ValErr>>(self)->Code();
}
gp::string thunk_SliceErr_Error(const gp::iface& self) {
    return gp::unbox<SliceErr>(self).Error();
}

const gp::method gp_pm_PtrErr[] = {
    gp::method_of<gp::string()>("Error", nullptr, "func() string", &thunk_pPtrErr_Error)};
const gp::method gp_vm_ValErr[] = {
    gp::method_of<int64_t()>("Code", nullptr, "func() int", &thunk_ValErr_Code),
    gp::method_of<gp::string()>("Error", nullptr, "func() string", &thunk_ValErr_Error)};
const gp::method gp_pm_ValErr[] = {
    gp::method_of<int64_t()>("Code", nullptr, "func() int", &thunk_pValErr_Code),
    gp::method_of<gp::string()>("Error", nullptr, "func() string", &thunk_pValErr_Error)};
const gp::method gp_vm_SliceErr[] = {
    gp::method_of<gp::string()>("Error", nullptr, "func() string", &thunk_SliceErr_Error)};
gp::string thunk_NoAssign_Error(const gp::iface& self) {
    return gp::unbox<NoAssign>(self).Error();
}
const gp::method gp_vm_NoAssign[] = {
    gp::method_of<gp::string()>("Error", nullptr, "func() string", &thunk_NoAssign_Error)};
const gp::imethod gp_im_Coder[] = {gp::imethod("Code", nullptr, "func() int")};

const gp::type_desc gp_td_PtrErr = gp::named<PtrErr>(
    "main.PtrErr", "main", gp::shape_struct(nullptr, 0), {}, gp::methods_of(gp_pm_PtrErr));
const gp::type_desc gp_td_ValErr =
    gp::named<ValErr>("main.ValErr", "main", gp::shape_struct(nullptr, 0),
                      gp::methods_of(gp_vm_ValErr), gp::methods_of(gp_pm_ValErr));
const gp::type_desc gp_td_SliceErr = gp::named<SliceErr>(
    "main.SliceErr", "main", gp::shape_struct(nullptr, 0), gp::methods_of(gp_vm_SliceErr));
const gp::type_desc gp_td_NoAssign = gp::named<NoAssign>(
    "main.NoAssign", "main", gp::shape_struct(nullptr, 0), gp::methods_of(gp_vm_NoAssign));
const gp::type_desc gp_td_NotErr =
    gp::named<NotErr>("main.NotErr", "main", gp::shape_struct(nullptr, 0));
const gp::type_desc gp_td_Coder =
    gp::named<Coder>("main.Coder", "main", gp::shape_interface(gp_im_Coder));
const gp::type_desc* coder_desc() {
    return &gp_td_Coder;
}

const gp::type_desc* td_pPtrErr() {
    static const gp::type_desc* const td = gp::td_ptr<PtrErr>(&gp_td_PtrErr);
    return td;
}
const gp::type_desc* td_pValErr() {
    static const gp::type_desc* const td = gp::td_ptr<ValErr>(&gp_td_ValErr);
    return td;
}
const gp::type_desc* td_pNotErr() {
    static const gp::type_desc* const td = gp::td_ptr<NotErr>(&gp_td_NotErr);
    return td;
}
const gp::type_desc* td_ppPtrErr() {
    static const gp::type_desc* const td = gp::td_ptr<gp::ptr<PtrErr>>(td_pPtrErr());
    return td;
}
const gp::type_desc* td_pCoder() {
    static const gp::type_desc* const td = gp::td_ptr<Coder>(&gp_td_Coder);
    return td;
}
const gp::type_desc* td_perror() {
    static const gp::type_desc* const td = gp::td_ptr<gp::error>(&gp::td_error);
    return td;
}
const gp::type_desc* td_pany() {
    static const gp::type_desc* const td = gp::td_ptr<gp::any>(&gp::td_any);
    return td;
}
const gp::type_desc* td_pint() {
    static const gp::type_desc* const td = gp::td_ptr<int64_t>(&gp::td_int);
    return td;
}

gp::error mk_ptr_err(const char* text) {
    return gp::to_iface<gp::error>(gp::new_<PtrErr>(PtrErr{gp::string::gp_copy(text)}),
                                   td_pPtrErr());
}
gp::error mk_val_err(int64_t code) {
    return gp::to_iface<gp::error>(ValErr{code}, &gp_td_ValErr);
}
gp::error mk_slice_err() {
    return gp::to_iface<gp::error>(SliceErr{gp::slice<gp::string>{}}, &gp_td_SliceErr);
}

// target(p, td) is the `any` that errors.As gets for &v: a pointer p to a variable, with the
// descriptor td of its pointer type.
template <typename T>
gp::any target(const gp::ptr<T>& p, const gp::type_desc* td) {
    return gp::to_iface<gp::any>(p, td);
}

void test_comparable() {
    CHECK(gp::errors::comparable(mk_ptr_err("a")));
    CHECK(gp::errors::comparable(mk_val_err(1)));
    CHECK(!gp::errors::comparable(mk_slice_err()));
    CHECK(!gp::errors::comparable(gp::error()));
    // A comparable error type compared with another type of error never panics: the types differ.
    CHECK((mk_val_err(1) == mk_ptr_err("a")) == false);
}

void test_check_target() {
    using namespace gp;
    // nil: the .gp source tests it first, the native one says the same
    CHECK(panic_text([] { errors::check_target(gp::any()); }) == "errors: target cannot be nil");
    // not a pointer
    CHECK(panic_text([] { errors::check_target(to_iface<any>(ValErr{1}, &gp_td_ValErr)); }) ==
          "errors: target must be a non-nil pointer");
    CHECK(panic_text([] { errors::check_target(to_iface<any>(int64_t(1), &td_int)); }) ==
          "errors: target must be a non-nil pointer");
    // a nil pointer (typed)
    CHECK(panic_text([] {
              errors::check_target(to_iface<any>(ptr<ptr<PtrErr>>(), td_ppPtrErr()));
          }) == "errors: target must be a non-nil pointer");
    // a pointer to something that is neither an interface nor an error
    ptr<NotErr> ne = new_<NotErr>(NotErr{});
    CHECK(panic_text([&] { errors::check_target(target(ne, td_pNotErr())); }) ==
          "errors: *target must be interface or implement error");
    ptr<int64_t> pi = new_<int64_t>(int64_t(0));
    CHECK(panic_text([&] { errors::check_target(target(pi, td_pint())); }) ==
          "errors: *target must be interface or implement error");
    // the good targets: *PtrErr's pointer, a pointer to ValErr (ValErr implements error), to
    // Coder, to error, to any
    ptr<ptr<PtrErr>> pp = new_<ptr<PtrErr>>();
    CHECK(panic_text([&] { errors::check_target(target(pp, td_ppPtrErr())); }) == "<no panic>");
    ptr<ValErr> pv = new_<ValErr>();
    CHECK(panic_text([&] { errors::check_target(target(pv, td_pValErr())); }) == "<no panic>");
    ptr<Coder> pc = new_<Coder>();
    CHECK(panic_text([&] { errors::check_target(target(pc, td_pCoder())); }) == "<no panic>");
    ptr<error> pe = new_<error>();
    CHECK(panic_text([&] { errors::check_target(target(pe, td_perror())); }) == "<no panic>");
    ptr<any> pa = new_<any>();
    CHECK(panic_text([&] { errors::check_target(target(pa, td_pany())); }) == "<no panic>");
}

void test_assign_concrete() {
    using namespace gp;
    // A *PtrErr target: the dynamic type must be *PtrErr, and the variable then shares the owner.
    ptr<ptr<PtrErr>> pp = new_<ptr<PtrErr>>();
    any t = target(pp, td_ppPtrErr());
    error other = mk_val_err(3);
    CHECK(!errors::assign(other, t) && *pp == nullptr);
    error e = mk_ptr_err("boom");
    CHECK(errors::assign(e, t));
    CHECK(*pp != nullptr && (*pp)->msg == "boom"_gs);
    CHECK((*pp).gp_shared() == e.gp_data()); // the same object: As gives the error itself
    // A value target: a copy of the boxed value.
    ptr<ValErr> pv = new_<ValErr>();
    any tv = target(pv, td_pValErr());
    CHECK(!errors::assign(mk_ptr_err("x"), tv) && pv->code == 0);
    CHECK(errors::assign(mk_val_err(42), tv) && pv->code == 42);
    CHECK(errors::assign(mk_val_err(7), tv) && pv->code == 7); // assigned again
    // A *PtrErr in the target of a ValErr: different types do not assign.
    CHECK(!errors::assign(mk_slice_err(), tv) && pv->code == 7);
}

void test_assign_without_store_hook() {
    using namespace gp;
    CHECK(gp_td_NoAssign.gp_store == nullptr && gp_td_ValErr.gp_store != nullptr);
    CHECK(td_pPtrErr()->gp_store != nullptr);
    const type_desc* tp = td_ptr<NoAssign>(&gp_td_NoAssign);
    ptr<NoAssign> p = new_<NoAssign>();
    any t = target(p, tp);
    errors::check_target(t); // a good target: NoAssign implements error
    error e = to_iface<error>(NoAssign("no copy"_gs), &gp_td_NoAssign);
    CHECK(panic_text([&] { (void)errors::assign(e, t); }) ==
          "errors: cannot assign to a *target of a C++ type without copy assignment");
    CHECK(p->text == ""_gs);
    // Another dynamic type is not an error of this function: false, no panic.
    CHECK(!errors::assign(mk_val_err(1), t));
}

void test_assign_interface() {
    using namespace gp;
    // Coder: ValErr has Code(), *PtrErr does not.
    ptr<Coder> pc = new_<Coder>();
    any tc = target(pc, td_pCoder());
    CHECK(!errors::assign(mk_ptr_err("x"), tc) && *pc == nullptr);
    error v = mk_val_err(9);
    CHECK(errors::assign(v, tc));
    CHECK(pc->gp_type() == &gp_td_ValErr && pc->gp_tab() != nullptr && pc->Code() == 9);
    CHECK(pc->gp_tab()->gp_inter == &gp_td_Coder);
    CHECK(pc->gp_data() == v.gp_data()); // the box is shared, boxes are never written
    // error: the target is an error variable (the itab is the error interface's)
    ptr<error> pe = new_<error>();
    any te = target(pe, td_perror());
    CHECK(errors::assign(mk_ptr_err("y"), te));
    CHECK(pe->gp_type() == td_pPtrErr() && pe->Error() == "y"_gs);
    // any: no itab
    ptr<any> pa = new_<any>();
    any ta = target(pa, td_pany());
    CHECK(errors::assign(mk_val_err(1), ta));
    CHECK(pa->gp_type() == &gp_td_ValErr && pa->gp_tab() == nullptr);
    // a nil error is assigned to nothing, whatever the target
    CHECK(!errors::assign(error(), tc) && !errors::assign(error(), ta) &&
          !errors::assign(error(), te));
    // the old value of the target is released: assign again with another error
    CHECK(errors::assign(mk_val_err(2), tc) && pc->Code() == 2);
}

} // namespace

int main() {
    test_comparable();
    test_check_target();
    test_assign_concrete();
    test_assign_without_store_hook();
    test_assign_interface();
    if (failures != 0) {
        return 1;
    }
    std::string msg = "ok: " + std::to_string(checks) + " checks\n";
    gp::os::write(gp::os::stdout_fd, msg.data(), msg.size());
    return 0;
}
