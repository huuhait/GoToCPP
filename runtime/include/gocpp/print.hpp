// gocpp/print.hpp: Go's builtins print and println.
//
// They write to standard error, in the number format of Go's runtime, which is not fmt's:
// println(1.5, 3.0, -0.25, 'x') prints "+1.500000e+000 +3.000000e+000 -2.500000e-001 120",
// and a pointer prints its address ("0x0" for nil). Each call is one write, and nothing is
// buffered, so output to stdout and stderr appears in the order the program writes it, like
// Go's. Slices and maps are a compile error (divergence D23), arrays and structs too (as in Go).
#pragma once

#include <gocpp/checks.hpp>
#include <gocpp/string.hpp>

#include <cstddef>
#include <cstdint>
#include <string>
#include <string_view>
#include <type_traits>

namespace gp {

namespace detail {

template <typename>
inline constexpr bool always_false = false;

// The pieces of the runtime's print format (Go's runtime/print.go). They append to out.
void print_int(std::string& out, int64_t v);
void print_uint(std::string& out, uint64_t v);
void print_float(std::string& out, double v); // "+1.500000e+000", "+Inf", "NaN"
void print_hex(std::string& out, uint64_t v); // "0x0", "0xc000012345"

// append_print appends v in the print format.
template <typename T>
void append_print(std::string& out, const T& v) {
    if constexpr (std::is_same_v<T, bool>) {
        out += v ? "true" : "false";
    } else if constexpr (go_int<T> && std::is_signed_v<T>) {
        print_int(out, static_cast<int64_t>(v));
    } else if constexpr (go_int<T>) {
        print_uint(out, static_cast<uint64_t>(v));
    } else if constexpr (go_float<T>) {
        print_float(out, static_cast<double>(v));
    } else if constexpr (std::is_same_v<T, gp::string>) {
        out += v.gp_view();
    } else if constexpr (std::is_pointer_v<T>) {
        print_hex(out, static_cast<uint64_t>(reinterpret_cast<uintptr_t>(v)));
    } else if constexpr (std::is_null_pointer_v<T>) {
        out += "0x0";
    } else if constexpr (requires { v.gp_shared(); }) {
        // A Go pointer (gp::ptr): its address, 0x0 for nil.
        print_hex(out, static_cast<uint64_t>(reinterpret_cast<uintptr_t>(v.gp_get())));
    } else if constexpr (requires { v.gp_value(); }) {
        append_print(out, v.gp_value()); // a named type (gp::newtype): as its underlying value
    } else {
        static_assert(always_false<T>, "print: a value of this type can't be printed yet");
    }
}

// write_stderr writes the bytes to standard error with one write.
void write_stderr(std::string_view s);

} // namespace detail

// print(args...): Go's print. It writes each argument with nothing between them.
template <typename... Ts>
void print(const Ts&... args) {
    if constexpr (sizeof...(Ts) > 0) {
        std::string out;
        (detail::append_print(out, args), ...);
        detail::write_stderr(out);
    }
}

// println(args...): Go's println. It writes the arguments with a space between them and a
// newline at the end.
template <typename... Ts>
void println(const Ts&... args) {
    std::string out;
    if constexpr (sizeof...(Ts) > 0) {
        std::size_t i = 0;
        ((i++ > 0 ? void(out += ' ') : void(), detail::append_print(out, args)), ...);
    }
    out += '\n';
    detail::write_stderr(out);
}

} // namespace gp
