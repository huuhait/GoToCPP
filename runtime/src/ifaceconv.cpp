// The panics of type assertions (gocpp/ifaceconv.hpp): the messages of Go's
// *runtime.TypeAssertionError (runtime/error.go). Standard C++ only.
#include <gocpp/rt.hpp>

#include <cstring>
#include <string>

namespace gp {
namespace {

// The package path of a type's descriptor, "" for a type without one.
const char* pkg_of(const type_desc* t) {
    return t->gp_pkg != nullptr ? t->gp_pkg : "";
}

[[noreturn]] void throw_assert(const std::string& text) {
    // The panic's value is a *runtime.TypeAssertionError whose Error() is the text (panic.hpp); a
    // new allocation in Go, so its identity is unique.
    detail::throw_type_assertion(gp::string::gp_copy(text));
}

} // namespace

void panic_assert(const type_desc* from, const type_desc* have, const type_desc* want) {
    std::string msg = "interface conversion: ";
    msg += from->gp_name;
    if (have == nullptr) {
        msg += " is nil, not ";
        msg += want->gp_name;
        throw_assert(msg);
    }
    msg += " is ";
    msg += have->gp_name;
    msg += ", not ";
    msg += want->gp_name;
    if (std::strcmp(have->gp_name, want->gp_name) == 0) {
        // Two types that print alike (Go: "types from different packages"/"scopes").
        msg += std::strcmp(pkg_of(have), pkg_of(want)) != 0 ? " (types from different packages)"
                                                            : " (types from different scopes)";
    }
    throw_assert(msg);
}

void panic_assert_iface(const type_desc* want, const type_desc* have) {
    std::string msg = "interface conversion: ";
    if (have == nullptr) {
        msg += "interface is nil, not ";
        msg += want->gp_name;
        throw_assert(msg);
    }
    msg += have->gp_name;
    msg += " is not ";
    msg += want->gp_name;
    msg += ": missing method ";
    const imethod* m = missing_method(want, have);
    msg += m != nullptr ? m->gp_name : "?";
    throw_assert(msg);
}

bool operator==(const iface& x, const iface& y) {
    const type_desc* t = x.gp_type();
    if (t != y.gp_type()) {
        return false;
    }
    if (t == nullptr) {
        return true;
    }
    if (t->gp_equal == nullptr) [[unlikely]] {
        throw panic_error{gp::string::gp_copy(
            std::string("runtime error: comparing uncomparable type ") + t->gp_name)};
    }
    return t->gp_equal(x.gp_data().get(), y.gp_data().get());
}

void iface::gp_key_check() const {
    if (gp_type_ != nullptr && gp_type_->gp_hash == nullptr) {
        throw panic_error{
            gp::string::gp_copy(std::string("hash of unhashable type: ") + gp_type_->gp_name)};
    }
}

std::size_t iface::gp_hash() const {
    const type_desc* t = gp_type_;
    if (t == nullptr) {
        return 0;
    }
    if (t->gp_hash == nullptr) [[unlikely]] {
        throw panic_error{gp::string::gp_copy(
            std::string("runtime error: hash of unhashable type ") + t->gp_name)};
    }
    // (1 as an int and as an int64 have one hash and are different keys: the == tells them apart.)
    return t->gp_hash(gp_data_.get());
}

} // namespace gp
