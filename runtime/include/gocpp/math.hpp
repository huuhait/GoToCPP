// gocpp/math.hpp: the native functions of the standard package math (Phase 09 step 9.5).
// Go's math package is ported (stdlib/math, tools/portstd); its .gp source calls these through
// //gocpp:native. The contract is Go's results on amd64, bit for bit
// (docs/adr/071-math-bit-exact.md):
//
//   - the functions that are exact in IEEE 754 are the C++ ones: sqrt, floor, ceil, trunc, fma and
//   abs
//     (they are inline, so that an optimized build is one instruction each);
//   - exp and log are Go's amd64 assembly, operation by operation (runtime/src/math.cpp), because
//   the
//     pure-Go versions give other bits for some inputs and Go on amd64 does not run them;
//   - the bit casts are std::bit_cast.
//
// None of these is noexcept: a func value of a native function (math.Sqrt as a value) needs a plain
// function type (gocpp/func.hpp). Nothing here is fused by the compiler: every compile has
// -ffp-contract=off (ADR-006), so a*b+c is two roundings, as in gc on amd64; the fusions of exp are
// explicit (__builtin_fma).
#pragma once

#include <bit>
#include <cstdint>

namespace gp::math {

inline uint64_t float64bits(double f) {
    return std::bit_cast<uint64_t>(f);
}

inline double float64frombits(uint64_t b) {
    return std::bit_cast<double>(b);
}

inline uint32_t float32bits(float f) {
    return std::bit_cast<uint32_t>(f);
}

inline float float32frombits(uint32_t b) {
    return std::bit_cast<float>(b);
}

// abs(x) clears the sign bit (NaN keeps its payload).
inline double abs(double x) {
    return std::bit_cast<double>(std::bit_cast<uint64_t>(x) & ~(uint64_t(1) << 63));
}

// sqrt(x) is the SQRTSD instruction: the correctly rounded square root, and NaN for x < 0. (Without
// the intrinsic, a compiler adds a call that sets errno for x < 0.)
inline double sqrt(double x) {
#if defined(__x86_64__)
    double r;
    asm("sqrtsd %1, %0" : "=x"(r) : "x"(x));
    return r;
#else
    return __builtin_sqrt(x);
#endif
}

// floor, ceil, trunc and round_to_even are ROUNDSD in Go on amd64 (SSE4.1, a runtime check), and
// here too when the CPU has it. Without it they are SSE2 code that gives the same bits for every
// input, so that they are inline: below 2^52 the integer part is the conversion to int64
// (truncation) and a correction; at 2^52 and above, an infinity and zero the number is its own
// result, and a NaN is made quiet, as ROUNDSD makes it (a signaling NaN with its payload becomes
// the quiet NaN with the same payload). The sign of a zero result follows x. Whether the CPU has
// SSE4.1 (Go's x86HasSSE41): then these are the instruction ROUNDSD itself.
extern const bool has_sse41;

namespace detail {
inline constexpr double two52 = 4503599627370496.0;

inline double trunc_small(double x) {
    return static_cast<double>(static_cast<int64_t>(x));
}

inline double other(double x) { // |x| >= 2^52, Inf or NaN
    return x != x ? x + x : x;
}

// The SSE2 versions (for a CPU without SSE4.1), callable on their own so that a test can compare
// them with ROUNDSD.
inline double floor_sse2(double x) {
    if (!(std::bit_cast<double>(std::bit_cast<uint64_t>(x) & ~(uint64_t(1) << 63)) < two52)) {
        return other(x);
    }
    double t = trunc_small(x);
    if (t > x) {
        t -= 1;
    }
    return __builtin_copysign(t, x);
}

inline double ceil_sse2(double x) {
    if (!(std::bit_cast<double>(std::bit_cast<uint64_t>(x) & ~(uint64_t(1) << 63)) < two52)) {
        return other(x);
    }
    double t = trunc_small(x);
    if (t < x) {
        t += 1;
    }
    return __builtin_copysign(t, x);
}

inline double trunc_sse2(double x) {
    if (!(std::bit_cast<double>(std::bit_cast<uint64_t>(x) & ~(uint64_t(1) << 63)) < two52)) {
        return other(x);
    }
    return __builtin_copysign(trunc_small(x), x);
}

inline double round_to_even_sse2(double x) {
    const double a = std::bit_cast<double>(std::bit_cast<uint64_t>(x) & ~(uint64_t(1) << 63));
    if (!(a < two52)) {
        return other(x);
    }
    // adding and subtracting 2^52 rounds to an integer, to nearest even in the default rounding
    // mode
    const double t = a + two52;
    return __builtin_copysign(t - two52, x);
}
} // namespace detail

inline double floor(double x) {
#if defined(__x86_64__)
    if (has_sse41) {
        double r;
        asm("roundsd $9, %1, %0" : "=x"(r) : "x"(x));
        return r;
    }
#endif
    return detail::floor_sse2(x);
}

inline double ceil(double x) {
#if defined(__x86_64__)
    if (has_sse41) {
        double r;
        asm("roundsd $10, %1, %0" : "=x"(r) : "x"(x));
        return r;
    }
#endif
    return detail::ceil_sse2(x);
}

inline double trunc(double x) {
#if defined(__x86_64__)
    if (has_sse41) {
        double r;
        asm("roundsd $11, %1, %0" : "=x"(r) : "x"(x));
        return r;
    }
#endif
    return detail::trunc_sse2(x);
}

// round_to_even(x) is ROUNDSD with the mode "nearest even".
inline double round_to_even(double x) {
#if defined(__x86_64__)
    if (has_sse41) {
        double r;
        asm("roundsd $8, %1, %0" : "=x"(r) : "x"(x));
        return r;
    }
#endif
    return detail::round_to_even_sse2(x);
}

// fma(x, y, z) is x*y+z with one rounding, as the instruction VFMADD231SD does in Go on amd64 (the
// same instruction, so that two NaN operands give the same NaN; without the instruction,
// __builtin_fma).
extern const bool has_fma;
inline double fma(double x, double y, double z) {
#if defined(__x86_64__)
    if (has_fma) {
        // Go's VFMADD231SD (dest z, vvvv x, rm y): AT&T order is rm, vvvv, dest.
        asm("vfmadd231sd %2, %1, %0" : "+x"(z) : "x"(x), "x"(y));
        return z;
    }
#endif
    return __builtin_fma(x, y, z);
}

// cpu_feature_disabled(godebug, name) is whether a GODEBUG value (a comma list) switches the CPU
// feature name ("fma", "avx") off with cpu.<name>=off or cpu.all=off, as Go's internal/cpu reads
// it.
bool cpu_feature_disabled(const char* godebug, const char* name);

// exp(x) is Go's archExp for amd64 (src/math/exp_amd64.s), with its two paths: the one with FMA
// instructions when the CPU has AVX and FMA (and GODEBUG has no cpu.fma=off, cpu.avx=off or
// cpu.all=off, as in Go), the other when it has not.
double exp(double x);

// exp_fma and exp_nofma are the two paths of exp, for the tests.
double exp_fma(double x);
double exp_nofma(double x);

// log(x) is Go's archLog for amd64 (src/math/log_amd64.s). For a subnormal x it is as wrong as
// Go's.
double log(double x);

} // namespace gp::math
