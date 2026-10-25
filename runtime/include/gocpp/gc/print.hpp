// gocpp/gc/print.hpp: Go's builtins print and println in the GC engine (phase-16 README §2.2;
// job 16.1c).
//
// They write to standard error, with one space between the arguments of println, a newline at
// the end of println, and nothing between the arguments of print. An int prints in decimal
// (Go's negative numbers, including the smallest int64), a bool as true or false, a string as
// its raw bytes. Each call is one write of the whole line, so the output appears in the order
// the program writes it.
//
// The subset is the one of job 16.1c: int, bool and string. Go's println also prints floats,
// complex numbers and pointers, and rejects structs, arrays, slices and maps at compile time;
// those come with the rest of the GC engine's values (step 16.5).
#pragma once

#include <gocpp/gc/string.hpp>

#include <cstddef>
#include <cstdint>
#include <string>
#include <string_view>
#include <type_traits>

namespace gp::gc {

namespace detail {

// print_int appends v to out in Go's print format: decimal, with '-' and the digits of the
// magnitude (also for INT64_MIN).
void print_int(std::string& out, int64_t v);

// write_stderr writes s to standard error in one call.
void write_stderr(std::string_view s);

#ifdef GOCPP_RUNTIME_TEST
// write_stderr_hook, when it is not null, receives every byte that write_stderr would put on
// standard error — print's and println's output, and the panic line (src/gc/panic.cpp and the
// run_main fallback in panic.hpp) — instead of writing it: the runtime's C++ tests
// (runtime/test/gc_print_test.cpp) check a call's
// exact output with it. Only a GOCPP_RUNTIME_TEST build has it (the runtime's tests, not a
// program).
extern void (*write_stderr_hook)(std::string_view);
#endif

// append_print appends one argument of print or println to out.
template <typename T>
    requires(std::is_integral_v<T> && !std::is_same_v<T, bool>)
inline void append_print(std::string& out, T v) {
    print_int(out, static_cast<int64_t>(v));
}
inline void append_print(std::string& out, bool v) {
    out += v ? "true" : "false";
}
inline void append_print(std::string& out, string v) {
    if (v.gp_len() > 0) {
        out.append(v.gp_data(), static_cast<std::size_t>(v.gp_len()));
    }
}
// A pointer must not convert to bool by accident: a C string is not a Go string, and a Go
// string literal is written gp::gc::lit("..."). This overload makes print("x") an error.
template <typename T>
    requires(std::is_pointer_v<T>)
void append_print(std::string& out, T v) = delete;

} // namespace detail

// print(args...): Go's print. It writes each argument with nothing between them.
template <typename... Ts>
void print(const Ts&... args) {
    std::string out;
    (detail::append_print(out, args), ...);
    detail::write_stderr(out);
}

// println(args...): Go's println. It writes the arguments with a space between them and a
// newline at the end.
template <typename... Ts>
void println(const Ts&... args) {
    std::string out;
    std::size_t i = 0;
    ((i++ > 0 ? void(out += ' ') : void(), detail::append_print(out, args)), ...);
    out += '\n';
    detail::write_stderr(out);
}

} // namespace gp::gc
