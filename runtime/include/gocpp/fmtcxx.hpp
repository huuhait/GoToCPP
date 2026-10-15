// gocpp/fmtcxx.hpp: fmt prints a C++ value through its std::formatter (Phase 04 README §2.12,
// step 4.12).
//
// Generated code that imports C/C++ headers includes this header after them (gocpp/rt.hpp
// comes before them): a type can have a std::formatter only when <format> came before, so
// this header uses it only then, and a program whose headers don't bring in <format> doesn't
// pay for it (about 0.2 s of compile time). __cpp_lib_format is defined by <format> (and by
// <version>, which then costs the include). Its own names start with gp_, so a macro of the
// program's headers can't break them.
#pragma once

#include <gocpp/fmtshim.hpp>

#if defined(__cpp_lib_format)
#include <format>
#include <iterator>
#include <string>
#include <type_traits>

namespace gp::fmtshim::detail {

// A disabled std::formatter (the C++ library's own for a type without one) can't be
// constructed ([format.formatter.spec]).
template <typename gp_T>
    requires std::is_default_constructible_v<std::formatter<gp_T, char>>
struct cxx_formatter<gp_T> {
    static constexpr bool gp_ok = true;
    static void gp_append(std::string& gp_out, const gp_T& gp_v) {
        std::format_to(std::back_inserter(gp_out), "{}", gp_v);
    }
};

} // namespace gp::fmtshim::detail
#endif
