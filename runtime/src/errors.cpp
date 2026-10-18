// The native functions of package errors (gocpp/errors.hpp, Phase 05 step 5.7). Standard C++
// only.
#include <gocpp/rt.hpp>

namespace gp::errors {
namespace {

[[noreturn]] void fail(const char* text) {
    gp::panic(gp::string::gp_copy(text));
}

} // namespace

bool comparable(const gp::iface& err) noexcept {
    const type_desc* t = err.gp_type();
    return t != nullptr && t->gp_equal != nullptr;
}

void check_target(const gp::iface& target) {
    const type_desc* t = target.gp_type();
    if (t == nullptr) {
        fail("errors: target cannot be nil");
    }
    if (t->gp_kind != kind::Pointer || target.gp_data().get() == nullptr) {
        fail("errors: target must be a non-nil pointer");
    }
    const type_desc* elem = t->gp_elem;
    if (elem->gp_kind != kind::Interface && !implements(&td_error, elem)) {
        fail("errors: *target must be interface or implement error");
    }
}

bool assign(const gp::iface& err, const gp::iface& target) {
    const type_desc* have = err.gp_type();
    if (have == nullptr) {
        return false; // a nil error is assignable to nothing (As skips them; this is no crash)
    }
    const type_desc* elem = target.gp_type()->gp_elem;
    void* dst = target.gp_data().get();
    if (elem->gp_kind == kind::Interface) {
        // The target is an interface variable: the value goes in with the itab of its type.
        const itab* tab = nullptr;
        if (elem->gp_nimethods != 0) {
            tab = getitab(elem, have);
            if (tab == nullptr) {
                return false;
            }
        }
        // (An interface struct has no data members of its own: it is an iface at its address.)
        *static_cast<iface*>(dst) = iface(err.gp_data(), have, tab);
        return true;
    }
    if (have != elem) {
        return false;
    }
    if (elem->gp_store == nullptr) [[unlikely]] {
        fail("errors: cannot assign to a *target of a C++ type without copy assignment");
    }
    elem->gp_store(dst, err.gp_data());
    return true;
}

} // namespace gp::errors
