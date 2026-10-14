// gocpp/fmtshim.hpp: gp::fmtshim::Print, Println and Printf for the basic types, a stand-in
// for Go's fmt until the real one (Phase 05 needs interfaces, Phase 09 completes it).
//
// Values print as fmt's %v does: bool as true/false, integers in decimal, strings as their
// bytes, and floats with the shortest digits that read back as the same float, in exponent
// form when the decimal exponent is < -4 or >= 6 (123456, 1e+06, 0.0001, 1e-05), and +Inf,
// -Inf, NaN. Printf knows the verbs %v %d %s %t %f and %%, and writes Go's error forms for
// the rest: %!d(string=x), %!d(MISSING), %!(EXTRA int=2), %!(NOVERB). Phase 03 adds width,
// precision, flags and more verbs.
//
// Each call formats everything first and then writes it with one write, and nothing is
// buffered, so the output order is Go's (Go's os.Stdout isn't buffered either).
//
// Until Phase 05 there are no type descriptors, so the type names in Go's error forms come
// from the C++ type: int64_t prints as "int" (it could be Go's int64) and uint64_t as "uint".
#pragma once

#include <gocpp/checks.hpp>
#include <gocpp/string.hpp>

#include <cstddef>
#include <cstdint>
#include <string>
#include <string_view>
#include <type_traits>

namespace gp::fmtshim {

namespace detail {

template <typename>
inline constexpr bool always_false = false;

// arg is one argument of a Print call, with its type erased, so the formatting code is not a
// template (it is in runtime/src/rt.cpp).
struct arg {
    enum kind_t : uint8_t { bool_, int_, uint_, float32_, float64_, string_ };
    kind_t kind;
    const char* type; // Go's type name, for the error forms: "int", "uint8", "string"
    union {
        bool b;
        int64_t i;
        uint64_t u;
        double f;
    };
    std::string_view s;
};

template <typename T>
constexpr const char* int_type_name() {
    if constexpr (std::is_signed_v<T>) {
        return sizeof(T) == 1   ? "int8"
               : sizeof(T) == 2 ? "int16"
               : sizeof(T) == 4 ? "int32"
                                : "int";
    } else {
        return sizeof(T) == 1   ? "uint8"
               : sizeof(T) == 2 ? "uint16"
               : sizeof(T) == 4 ? "uint32"
                                : "uint";
    }
}

template <typename T>
arg make_arg(const T& v) {
    arg a{};
    if constexpr (std::is_same_v<T, bool>) {
        a.kind = arg::bool_;
        a.type = "bool";
        a.b = v;
    } else if constexpr (go_int<T> && std::is_signed_v<T>) {
        a.kind = arg::int_;
        a.type = int_type_name<T>();
        a.i = static_cast<int64_t>(v);
    } else if constexpr (go_int<T>) {
        a.kind = arg::uint_;
        a.type = int_type_name<T>();
        a.u = static_cast<uint64_t>(v);
    } else if constexpr (std::is_same_v<T, float>) {
        a.kind = arg::float32_;
        a.type = "float32";
        a.f = static_cast<double>(v);
    } else if constexpr (std::is_same_v<T, double>) {
        a.kind = arg::float64_;
        a.type = "float64";
        a.f = v;
    } else if constexpr (std::is_same_v<T, gp::string>) {
        a.kind = arg::string_;
        a.type = "string";
        a.s = v.gp_view();
    } else {
        static_assert(always_false<T>,
                      "fmtshim: a value of this type can't be printed yet (Phase 03)");
    }
    return a;
}

// float_v and float_f append a float as %v and as %f (6 digits after the point) do. bits is
// 32 for a float32, whose shortest digits differ: float32(0.1) prints as 0.1.
void float_v(std::string& out, double v, int bits);
void float_f(std::string& out, double v);

// The formatting, as Go's fmt.Sprint, Sprintln and Sprintf do it. They append to out.
void sprint(std::string& out, const arg* args, std::size_t n);
void sprintln(std::string& out, const arg* args, std::size_t n);
void sprintf(std::string& out, std::string_view format, const arg* args, std::size_t n);

// write_stdout writes the bytes to standard output with one write.
void write_stdout(std::string_view s);

} // namespace detail

// Print is fmt.Print: it adds a space between two arguments when neither is a string.
template <typename... Ts>
void Print(const Ts&... args) {
    std::string out;
    if constexpr (sizeof...(Ts) > 0) {
        const detail::arg a[] = {detail::make_arg(args)...};
        detail::sprint(out, a, sizeof...(Ts));
    }
    detail::write_stdout(out);
}

// Println is fmt.Println: a space between all arguments, and a newline at the end.
template <typename... Ts>
void Println(const Ts&... args) {
    std::string out;
    if constexpr (sizeof...(Ts) > 0) {
        const detail::arg a[] = {detail::make_arg(args)...};
        detail::sprintln(out, a, sizeof...(Ts));
    } else {
        detail::sprintln(out, nullptr, 0);
    }
    detail::write_stdout(out);
}

// Printf is fmt.Printf with the verbs %v %d %s %t %f and %%.
template <typename... Ts>
void Printf(const gp::string& format, const Ts&... args) {
    std::string out;
    if constexpr (sizeof...(Ts) > 0) {
        const detail::arg a[] = {detail::make_arg(args)...};
        detail::sprintf(out, format.gp_view(), a, sizeof...(Ts));
    } else {
        detail::sprintf(out, format.gp_view(), nullptr, 0);
    }
    detail::write_stdout(out);
}

} // namespace gp::fmtshim
