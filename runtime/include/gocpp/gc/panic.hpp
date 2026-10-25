// gocpp/gc/panic.hpp: the GC engine's panics until step 16.6 (phase-16 README §2.2 and §2.6;
// job 16.1e).
//
// A panic in the GC engine throws gp::gc::panic_error, which carries Go's message and the
// dynamic type that Go's runtime error has ("runtime.errorString", "runtime.boundsError",
// "runtime.plainError"). This is a placeholder, not the shared mechanism: the ARC engine's
// panic_error (gocpp/panic.hpp) holds a gp::any and a stack trace and drags the reference-
// counting runtime in, so the GC engine cannot use it without linking ARC values. Step 16.6
// replaces this type with one exception type for both engines, with defers, recover, Goexit and
// the stack trace, and makes the panic value a Go value rooted in the GC heap (README §2.4:
// C++ exception objects live in malloc memory, which the collector does not scan).
//
// What step 16.6 must unify, precisely:
//   - one exception type for both engines (the plan's choice), carrying Go's panic value, so a
//     panic unwinds through frames of both engines and recover() returns the same value;
//   - the GC engine's runtime errors as the runtime.Error types Go has (runtime.boundsError,
//     runtime.errorString, runtime.plainError), so errors.As and the Error() text agree;
//   - the program entry: an uncaught panic prints "panic: <v>", the goroutine line and the
//     stack trace, and exits 2, as gp::run_main does for ARC. gp::gc::run_main is the GC
//     engine's entry point for that (job 16.1f's emitter should call it); until then an
//     uncaught panic reaches std::terminate, and the runtime's tests catch panic_error.
#pragma once

#include <gocpp/gc/print.hpp>

#include <exception>
#include <string>
#include <utility>

namespace gp::gc {

// panic_error is the value of a GC-engine panic: Go's text (with the "runtime error: " prefix
// that Go's runtime.Error values print) and Go's dynamic type of the panic value. It does not
// derive from std::exception, so C++ code that catches std::exception cannot stop a Go panic by
// accident.
struct panic_error {
    std::string
        gp_message; // Go's text, for example "runtime error: index out of range [5] with length 3"
    std::string gp_type; // Go's dynamic type of the panic value, for example "runtime.boundsError"
};

namespace detail {

// panic_hook, when it is not null, receives each panic instead of the default handling: a
// program entry that wants C++ exceptions (step 16.6, or the runtime's own tests, which throw
// and catch panic_error) sets it. It is zero-initialized static data, so it adds no static
// initializer. Until 16.6, a program that does not set it gets job 16.1c's behavior: Go's panic
// line on standard error and exit status 2.
extern void (*panic_hook)(const panic_error&);

} // namespace detail

// panic handles a panic: detail::panic_hook if it is set, else "panic: <message>" on standard
// error and exit status 2 (Go's line, before the stack trace of step 16.6).
[[noreturn]] void panic(std::string message, const char* type = "runtime.errorString");

// panic_runtime throws a runtime error: Go's message with the "runtime error: " prefix.
[[noreturn]] void panic_runtime(std::string message, const char* type = "runtime.errorString");

// run_main runs the program's main function with Go's handling of an uncaught panic: it prints
// "panic: <message>" to standard error and returns 2. The stack trace, the goroutine line and
// the panic chain are step 16.6.
template <typename F>
int run_main(F&& f) {
    try {
        std::forward<F>(f)();
        return 0;
    } catch (const panic_error& p) {
        detail::write_stderr(std::string("panic: ") + p.gp_message + "\n");
        return 2;
    }
}

} // namespace gp::gc
