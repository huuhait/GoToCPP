// gocpp/gc/panic.cpp: the GC engine's panics, with Go's exact messages (phase-16 README §2.2;
// job 16.1e). The declarations and what step 16.6 must unify are in gocpp/gc/panic.hpp and
// gocpp/gc/checks.hpp.
#include <gocpp/nofortify.h>
#include <gocpp/gc/checks.hpp>
#include <gocpp/gc/panic.hpp>

#include <cstdint>
#include <cstdio>
#include <cstdlib>
#include <string>
#include <utility>

namespace gp::gc {

namespace detail {

// Zero-initialized static data: no static initializer (the runtime's objects must have none).
void (*panic_hook)(const panic_error&) = nullptr;

} // namespace detail

namespace {

// bounds_fmts and bounds_neg_fmts are Go's boundsErrorFmts and boundsNegErrorFmts
// (runtime/error.go), in the order of gp::gc::bounds. %x is the index that failed, %y what it
// was compared with. The second table is for a negative signed %x, which Go prints without %y.
constexpr const char* bounds_fmts[] = {
    "slice bounds out of range [:%x] with length %y",
    "slice bounds out of range [:%x] with capacity %y",
    "slice bounds out of range [%x:%y]",
    "slice bounds out of range [::%x] with length %y",
    "slice bounds out of range [::%x] with capacity %y",
    "slice bounds out of range [:%x:%y]",
    "slice bounds out of range [%x:%y:]",
    "cannot convert slice with length %y to array or pointer to array with length %x",
};

constexpr const char* bounds_neg_fmts[] = {
    "slice bounds out of range [:%x]",
    "slice bounds out of range [:%x]",
    "slice bounds out of range [%x:]",
    "slice bounds out of range [::%x]",
    "slice bounds out of range [::%x]",
    "slice bounds out of range [:%x:]",
    "slice bounds out of range [%x::]",
    "cannot convert slice with length %y to array or pointer to array with length %x",
};

// append_number appends v to text in Go's decimal format: a signed value with '-', an unsigned
// one as it is.
void append_number(std::string& text, std::int64_t v, bool is_signed) {
    if (is_signed) {
        text += std::to_string(v);
    } else {
        text += std::to_string(static_cast<std::uint64_t>(v));
    }
}

} // namespace

void panic(std::string message, const char* type) {
    panic_error p{std::move(message), type};
    if (detail::panic_hook != nullptr) {
        detail::panic_hook(p); // step 16.6's exception path, or the tests
    }
    // Job 16.1c's behavior: Go's panic line and exit status 2 (the stack trace is step 16.6).
    std::fprintf(stderr, "panic: %s\n", p.gp_message.c_str());
    std::fflush(stderr);
    std::_Exit(2);
}

void panic_runtime(std::string message, const char* type) {
    panic("runtime error: " + message, type);
}

void panic_index(std::int64_t x, bool is_signed, std::int64_t len) {
    std::string text = "runtime error: index out of range [";
    append_number(text, x, is_signed);
    text += ']';
    if (!(is_signed && x < 0)) {
        text += " with length ";
        append_number(text, len, true);
    }
    panic(std::move(text), "runtime.boundsError");
}

void panic_bounds(bounds code, std::int64_t x, bool is_signed, std::int64_t y) {
    auto i = static_cast<std::size_t>(code);
    const char* fmt = is_signed && x < 0 ? bounds_neg_fmts[i] : bounds_fmts[i];
    std::string text = "runtime error: ";
    for (const char* p = fmt; *p != '\0'; p++) {
        if (*p != '%') {
            text += *p;
            continue;
        }
        p++;
        if (*p == 'x') {
            append_number(text, x, is_signed);
        } else {
            append_number(text, y, true);
        }
    }
    panic(std::move(text), "runtime.boundsError");
}

void panic_nil() {
    panic_runtime("invalid memory address or nil pointer dereference");
}

void panic_divide() {
    panic_runtime("integer divide by zero");
}

void panic_shift() {
    panic_runtime("negative shift amount");
}

void panic_makeslicelen() {
    panic_runtime("makeslice: len out of range");
}

void panic_makeslicecap() {
    panic_runtime("makeslice: cap out of range");
}

void panic_growslice() {
    panic_runtime("growslice: len out of range");
}

void panic_uncomparable(const char* name) {
    panic_runtime(std::string("comparing uncomparable type ") + name);
}

void panic_nil_map() {
    // A runtime.plainError: no "runtime error: " prefix (Go prints "assignment to entry in nil
    // map").
    panic("assignment to entry in nil map", "runtime.plainError");
}

} // namespace gp::gc
