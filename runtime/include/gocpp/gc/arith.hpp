// gocpp/gc/arith.hpp: the integer operations whose C++ operators do not have Go's meaning
// (phase-16 README §2.2; job 16.1c).
//
// Go's shifts have no upper limit on the count: x << 64 is 0 and x >> 64 is 0 or -1, where C++
// shifts by the width or more are undefined. Go's division and remainder panic for a zero
// divisor and define INT64_MIN / -1 as INT64_MIN (and the remainder as 0), where C++ leaves both
// undefined. The generated code calls these functions for <<, >>, / and %.
//
// The panic is a placeholder of job 16.1c: it prints the first line of Go's message and exits
// with status 2. Go's real panic (the shared exception, defers, a stack trace) is step 16.6, and
// replaces this file's spike_panic; it is marked so that it is replaced, not extended.
#pragma once

#include <cstdint>

namespace gp::gc {

// shl returns x << n with Go's meaning: n must not be negative (a panic), and a count of 64 or
// more gives 0.
int64_t shl(int64_t x, int64_t n);

// shr returns x >> n with Go's meaning: an arithmetic shift (the sign fills), n must not be
// negative, and a count of 64 or more gives 0 for a non-negative x and -1 for a negative one.
int64_t shr(int64_t x, int64_t n);

// div returns x / y with Go's meaning: a zero divisor panics, and INT64_MIN / -1 is INT64_MIN.
int64_t div(int64_t x, int64_t y);

// rem returns x % y with Go's meaning: a zero divisor panics, and INT64_MIN % -1 is 0.
int64_t rem(int64_t x, int64_t y);

} // namespace gp::gc
