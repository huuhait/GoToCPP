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
// followed by a tab. When the goroutine has a signal (below), Go's signal line comes after the
// text, before the blank line. Go then prints the stack trace (Phase 03). Step 3.13 passes the
// whole panic chain (a panic in a deferred call adds "\tpanic: ..." lines); the signal line
// still comes once, after the chain, as in Go.
void panic_report(std::string& out, const gp::string& text);

// The goroutine's signal, Go's g.sig: in Go, a nil pointer dereference is a memory fault, and
// an unrecovered panic then prints a signal line, "[signal SIGSEGV: segmentation violation
// code=0x1 addr=0x0 pc=0x4a2b1c]". panic_nil sets it for the running thread (a goroutine is a
// thread, ADR-007), and it stays set while the panic goes on, also through panics that
// deferred calls add. A recover that stops the panic must call clear_signal (Phase 05), as Go
// clears g.sig; a later, ordinary panic then prints no signal line.
void clear_signal() noexcept;

} // namespace detail

} // namespace gp
