// gocpp/panic.hpp: Go's panic as a C++ exception, gp::panic_error.
//
// panic(v) throws gp::panic_error. gp::run_main catches it, prints "panic: <v>" and
// "goroutine 1 [running]:" to stderr and exits with code 2, like Go. Because a panic unwinds
// the C++ stack, the defers of Phase 03 can run during a panic without any change to the call
// sites. Phase 05 makes the value a gp::any, for recover().
#pragma once

#include <gocpp/checks.hpp>
#include <gocpp/print.hpp>
#include <gocpp/string.hpp>

#include <string>
#include <type_traits>

namespace gp {

// panic_error is the exception that a Go panic throws. It doesn't derive from std::exception,
// so C++ code that catches std::exception can't stop a Go panic by accident.
struct panic_error {
    // The panic value as Go prints it after "panic: ": a string as it is, a number or a bool
    // in the format of the builtin print, a runtime error as "runtime error: ...".
    gp::string gp_text;
};

// panic(v): Go's builtin panic for a value of a basic type. A string is the text as it is; a
// bool or a number prints like the builtin print does (panic(1.5) prints
// "panic: +1.500000e+000", as in Go).
[[noreturn]] void panic(const gp::string& v);

template <typename T>
    requires std::is_same_v<T, bool> || go_int<T> || go_float<T>
[[noreturn]] void panic(const T& v) {
    std::string text;
    detail::append_print(text, v);
    panic(gp::string::gp_copy(text));
}

namespace detail {

// panic_report appends what Go prints for an unrecovered panic with this text:
// "panic: " + text + "\n\ngoroutine 1 [running]:\n". As in Go, each newline in the text is
// followed by a tab. Go then prints the stack trace (Phase 03).
void panic_report(std::string& out, const gp::string& text);

} // namespace detail

} // namespace gp
