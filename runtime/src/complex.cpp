// The part of complex numbers (gocpp/complex.hpp) that is not a template: the division, Go's
// runtime.complex128div with the float helpers of Go's runtime/float.go, line by line, so
// that every result is Go's (also for infinities, NaN and zeros). Compiled with
// -ffp-contract=off, like everything (the driver's flags).
#include <gocpp/rt.hpp>

#include <cstdint>
#include <cstring>
#include <limits>

namespace gp {

namespace {

constexpr double inf = std::numeric_limits<double>::infinity();

bool is_nan(double f) {
    return f != f;
}

bool is_finite(double f) {
    return !is_nan(f - f);
}

bool is_inf(double f) {
    return !is_nan(f) && !is_finite(f);
}

uint64_t bits(double f) {
    uint64_t b;
    std::memcpy(&b, &f, sizeof b);
    return b;
}

double from_bits(uint64_t b) {
    double f;
    std::memcpy(&f, &b, sizeof f);
    return f;
}

constexpr uint64_t sign_bit = uint64_t(1) << 63;

double abs(double x) {
    return from_bits(bits(x) & ~sign_bit);
}

double copysign(double x, double y) {
    return from_bits((bits(x) & ~sign_bit) | (bits(y) & sign_bit));
}

// inf2one returns a signed 1 if f is an infinity and a signed 0 otherwise. The sign of the
// result is the sign of f.
double inf2one(double f) {
    double g = 0.0;
    if (is_inf(f)) {
        g = 1.0;
    }
    return copysign(g, f);
}

} // namespace

complex128 complex128div(complex128 n, complex128 m) noexcept {
#if defined(__clang__)
#pragma STDC FP_CONTRACT OFF // as in operator* (gocpp/complex.hpp)
#endif
    double e, f; // complex(e, f) = n/m

    // Algorithm for robust complex division as described in Robert L. Smith: Algorithm 116:
    // Complex division. Commun. ACM 5(8): 435 (1962).
    if (abs(m.gp_re) >= abs(m.gp_im)) {
        double ratio = m.gp_im / m.gp_re;
        double denom = m.gp_re + ratio * m.gp_im;
        e = (n.gp_re + n.gp_im * ratio) / denom;
        f = (n.gp_im - n.gp_re * ratio) / denom;
    } else {
        double ratio = m.gp_re / m.gp_im;
        double denom = m.gp_im + ratio * m.gp_re;
        e = (n.gp_re * ratio + n.gp_im) / denom;
        f = (n.gp_im * ratio - n.gp_re) / denom;
    }

    if (is_nan(e) && is_nan(f)) {
        // Correct final result to infinities and zeros if applicable. Matches C99: ISO/IEC
        // 9899:1999 - G.5.1 Multiplicative operators.
        double a = n.gp_re, b = n.gp_im;
        double c = m.gp_re, d = m.gp_im;

        if (m == complex128() && (!is_nan(a) || !is_nan(b))) {
            e = copysign(inf, c) * a;
            f = copysign(inf, c) * b;
        } else if ((is_inf(a) || is_inf(b)) && is_finite(c) && is_finite(d)) {
            a = inf2one(a);
            b = inf2one(b);
            e = inf * (a * c + b * d);
            f = inf * (b * c - a * d);
        } else if ((is_inf(c) || is_inf(d)) && is_finite(a) && is_finite(b)) {
            c = inf2one(c);
            d = inf2one(d);
            e = 0 * (a * c + b * d);
            f = 0 * (b * c - a * d);
        }
    }

    return complex128(e, f);
}

} // namespace gp
