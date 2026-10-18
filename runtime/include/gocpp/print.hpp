// gocpp/print.hpp: Go's builtins print and println.
//
// They write to standard error, in the number format of Go's runtime, which is not fmt's:
// println(1.5, 3.0, -0.25, 'x') prints "+1.500000e+000 +3.000000e+000 -2.500000e-001 120",
// a complex number prints as "(+1.000000e+000+2.000000e+000i)", and a pointer prints its
// address ("0x0" for nil). Each call is one write, and nothing is buffered, so output to stdout
// and stderr appears in the order the program writes it, like Go's; C's stdout and stderr are
// flushed first (flush_c_stdio), so what C code printf'd before comes out before. Slices and
// maps are a compile error (divergence D23), arrays and structs too (as in Go).
//
// C/C++ values (Phase 04 README §2.12, step 4.12) come through gp::cgo::print_arg
// (gocpp/interop.hpp), which reports a type that print can't print at the .gp line: C/C++
// numbers and bools print as the Go type of the same kind (a C.char is a number), an enum as
// its number, any raw pointer (also a char* and a function pointer) as its address, nullptr_t
// as 0x0, std::string and std::string_view as strings. Other C++ objects are an error, as
// structs are in Go: fmt.Println prints them.
#pragma once

#include <gocpp/checks.hpp>
#include <gocpp/complex.hpp>
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

// print_ok: print and println can print a T (a C/C++ value through gp::cgo::print_arg).
template <typename T>
inline constexpr bool print_ok =
    std::is_same_v<T, bool> || go_int<T> || go_float<T> || go_complex<T> ||
    std::is_same_v<T, gp::string> || std::is_pointer_v<T> || std::is_null_pointer_v<T> ||
    std::is_enum_v<T> || std::is_same_v<T, std::string> || std::is_same_v<T, std::string_view> ||
    requires(const T& v) { v.gp_shared(); } || requires(const T& v) { v.gp_address(); };

// cxx_print is a value with a C/C++ part, from gp::cgo::print_arg, which reports one that
// print can't print; that one then prints nothing, so that the error isn't reported twice.
template <typename T>
struct cxx_print {
    using gp_cxx_print = void;
    T gp_v;
};

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
    } else if constexpr (go_complex<T>) {
        // Go's printcomplex: "(", the parts as floats, "i)": (+1.000000e+000-2.000000e+000i).
        out += '(';
        print_float(out, static_cast<double>(v.gp_re));
        print_float(out, static_cast<double>(v.gp_im));
        out += "i)";
    } else if constexpr (std::is_same_v<T, gp::string>) {
        out += v.gp_view();
    } else if constexpr (std::is_pointer_v<T>) {
        print_hex(out, static_cast<uint64_t>(reinterpret_cast<uintptr_t>(v)));
    } else if constexpr (std::is_null_pointer_v<T>) {
        out += "0x0";
    } else if constexpr (std::is_enum_v<T>) {
        append_print(out, static_cast<std::underlying_type_t<T>>(v)); // a C/C++ enum
    } else if constexpr (std::is_same_v<T, std::string> || std::is_same_v<T, std::string_view>) {
        out += v;
    } else if constexpr (requires { v.gp_shared(); }) {
        // A Go pointer (gp::ptr): its address, 0x0 for nil.
        print_hex(out, static_cast<uint64_t>(reinterpret_cast<uintptr_t>(v.gp_get())));
    } else if constexpr (requires { v.gp_address(); }) {
        // A channel (gp::chan): its address, 0x0 for nil.
        print_hex(out, static_cast<uint64_t>(v.gp_address()));
    } else if constexpr (requires { v.gp_value(); }) {
        append_print(out, v.gp_value()); // a named type (gp::newtype): as its underlying value
    } else if constexpr (requires { typename T::gp_cxx_print; }) {
        if constexpr (print_ok<decltype(v.gp_v)>) {
            append_print(out, v.gp_v);
        }
    } else {
        static_assert(always_false<T>, "print: a value of this type can't be printed yet");
    }
}

// flush_c_stdio flushes C's stdout and stderr (fflush), so that what C or C++ code printf'd
// comes out before what gocpp writes next: C's stdout has a buffer when it is no terminal,
// gocpp's writes have none. It costs about 3 ns per stream when there is nothing to flush.
void flush_c_stdio() noexcept;

// write_stderr writes the bytes to standard error with one write, after flush_c_stdio.
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
