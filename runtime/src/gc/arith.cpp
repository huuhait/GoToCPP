// gocpp/gc/arith.cpp: Go's shifts and division/remainder (phase-16 README §2.2; job 16.1c).
#include <gocpp/nofortify.h>
#include <gocpp/gc/arith.hpp>

#include <cstdio>
#include <cstdlib>

namespace gp::gc {

namespace {

// spike_panic is job 16.1c's placeholder for a Go runtime panic: it prints the first line of
// Go's message and exits with status 2, which is what a Go program that panics prints before its
// stack trace. It is not the panic mechanism: the shared exception type, defers, recover and the
// stack trace are step 16.6, which replaces this function (job 16.1c's report says so).
[[noreturn]] void spike_panic(const char* msg) {
    std::fprintf(stderr, "panic: runtime error: %s\n", msg);
    std::fflush(stderr);
    std::_Exit(2);
}

} // namespace

int64_t shl(int64_t x, int64_t n) {
    if (n < 0) {
        spike_panic("negative shift amount");
    }
    if (n >= 64) {
        return 0;
    }
    return static_cast<int64_t>(static_cast<std::uint64_t>(x) << static_cast<unsigned>(n));
}

int64_t shr(int64_t x, int64_t n) {
    if (n < 0) {
        spike_panic("negative shift amount");
    }
    if (n >= 64) {
        return x < 0 ? -1 : 0;
    }
    // C++20 defines the right shift of a signed value as an arithmetic shift (the sign fills).
    return x >> n;
}

int64_t div(int64_t x, int64_t y) {
    if (y == 0) {
        spike_panic("integer divide by zero");
    }
    if (y == -1) {
        // INT64_MIN / -1 is INT64_MIN in Go, and undefined in C++: the negation in uint64_t
        // gives Go's value for every x (INT64_MIN for INT64_MIN).
        return static_cast<int64_t>(std::uint64_t{0} - static_cast<std::uint64_t>(x));
    }
    return x / y;
}

int64_t rem(int64_t x, int64_t y) {
    if (y == 0) {
        spike_panic("integer divide by zero");
    }
    if (y == -1) {
        return 0; // INT64_MIN % -1 is 0 in Go, and undefined in C++
    }
    return x % y;
}

} // namespace gp::gc
