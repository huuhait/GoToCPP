// gocpp/error.hpp: Go's predeclared error type (Phase 05 §2.5, step 5.4), `interface{ Error()
// string }`, as the interface struct gp::error, with its descriptor gp::td_error ("error").
//
// It is an interface struct like the ones the emitter writes for a Go interface type
// (docs/spec/cpp-mapping.md, "Interfaces"): no data members of its own (32 bytes), the
// descriptor, and one wrapper per method that calls the itab's slot. A nil error is
// `gp::error()`. The errors package (step 5.7) and fmt (step 5.11) build on it.
#pragma once

#include <gocpp/iface.hpp>

namespace gp {

struct error : iface {
    using iface::iface;
    constexpr error() noexcept = default;
    static const type_desc* gp_desc() noexcept;
    gp::string Error() const;
};

static_assert(sizeof(error) == sizeof(iface), "gp::error has no members of its own");

namespace detail {
inline constexpr imethod error_methods[] = {imethod("Error", nullptr, "func() string")};
} // namespace detail

// td_error is the descriptor of `error`: a named interface type with no package.
inline constexpr type_desc td_error =
    named<error>("error", nullptr, shape_interface(detail::error_methods));

inline const type_desc* error::gp_desc() noexcept {
    return &td_error;
}

inline gp::string error::Error() const {
    return icall<gp::string()>(*this, 0);
}

} // namespace gp
