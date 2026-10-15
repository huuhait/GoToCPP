// gocpp/complex.hpp: Go's complex64 and complex128 (Phase 03, step 3.15c).
//
// gp::complex64 and gp::complex128 are gp::complex<float> and gp::complex<double>: the real
// part, then the imaginary part (8 bytes, alignment 4, and 16 bytes, alignment 8, as in Go).
// They are not std::complex, whose * and / follow C's Annex G (they recover infinities after
// the multiplication too) and whose results differ between libraries; these have Go's
// operations, each written as Go's compiler (gc on amd64, GOAMD64=v1) writes it:
//
//     Go               C++                       what it computes
//     a + b, a - b     a + b, a - b              each part, in the parts' own type (float32 for
//                                                complex64)
//     a * b            a * b                     (ar*br - ai*bi) + (ar*bi + ai*br)i, always in
//                                                float64 (a complex64 is widened first, and the
//                                                parts rounded back), with no fused
//                                                multiply-add and no recovery of infinities:
//                                                (Inf+0i) * 2 is (+Inf+NaNi), as in Go
//     a / b            a / b                     runtime.complex128div (runtime/src/complex.cpp):
//                                                Smith's algorithm with C99's corrections when both
//                                                parts are NaN; a complex64 is divided as a
//                                                complex128 and rounded back. A division by zero
//                                                gives infinities or NaN and never panics
//     -a               -a                        each part negated (so -(0+0i) is (-0-0i))
//     a == b, a != b   a == b, a != b            both parts compare equal (a NaN part never does)
//     c++, c += 1      c++, c = c + ...          adds (1+0i): the imaginary part gets + 0, so a -0
//                                                part becomes +0, as in Go
//     complex(r, i)    gp::complex128(r, i)
//     real(c), imag(c) gp::real(c), gp::imag(c)
//     complex64(c)     gp::complex64(c)          each part converted (rounded to float32)
//
// The emitter writes a constant as gp::complex128(1.0, 2.0) (go/types folds every constant
// expression exactly; a complex64 constant has float32 parts, 1.5f). The constructors are
// constexpr, so a package variable with a constant value is constinit.
//
// The program must be compiled with -ffp-contract=off (the driver's flags), or the C++
// compiler may fuse ar*br - ai*bi into one operation, which rounds differently. gocpp gives
// gc's results on amd64 at GOAMD64=v1 and v2, without fusing, everywhere: from v3 on, and on
// arm64, gc itself fuses a*b + c, and its results can differ in the last bit. -ffast-math
// would break the NaN and infinity rules too. For C++ files built with other flags (Phase
// 04), operator* and complex128div also say #pragma STDC FP_CONTRACT OFF for their own block,
// which clang honors (also at its default, -ffp-contract=on, but not with =fast); GCC doesn't
// know the pragma (it would warn), so only clang sees it, and GCC needs the flag. The sign and
// the payload of a NaN that an operation makes are not promised (Go doesn't promise them
// either: they are the CPU's).
//
// Map keys hash both parts with the float rules (gocpp/hash.hpp: +0 and -0 hash the same, a
// NaN key is never found again). println prints Go's runtime format,
// (+1.000000e+000+2.000000e+000i). Like every runtime type, gp::complex has only operators and
// gp_-prefixed members.
#pragma once

#include <gocpp/checks.hpp>
#include <gocpp/hash.hpp>

#include <cstddef>
#include <cstdint>
#include <type_traits>

namespace gp {

template <go_float F>
struct complex;

using complex64 = complex<float>;
using complex128 = complex<double>;

// complex128div(n, m) is Go's runtime.complex128div: n / m (runtime/src/complex.cpp).
complex128 complex128div(complex128 n, complex128 m) noexcept;

template <go_float F>
struct complex {
    F gp_re{};
    F gp_im{};

    constexpr complex() = default;
    constexpr complex(F re, F im) noexcept : gp_re(re), gp_im(im) {}
    // complex(re): re + 0i, for newtype's c + 1 (gocpp/newtype.hpp).
    constexpr explicit complex(F re) noexcept : gp_re(re), gp_im(0) {}
    // complex64(c), complex128(c): Go's conversion, part by part.
    template <go_float G>
        requires(!std::is_same_v<F, G>)
    constexpr explicit complex(const complex<G>& c) noexcept
        : gp_re(static_cast<F>(c.gp_re)), gp_im(static_cast<F>(c.gp_im)) {}

    // Like Go's c128hash: the real part, then the imaginary part, in order (hash_combine alone is
    // symmetric, so (1+2i) and (2+1i) would collide).
    std::size_t gp_hash() const noexcept { return gp::hash_fields(gp_re, gp_im); }

    friend constexpr complex operator+(const complex& a, const complex& b) noexcept {
        return complex(a.gp_re + b.gp_re, a.gp_im + b.gp_im);
    }
    friend constexpr complex operator-(const complex& a, const complex& b) noexcept {
        return complex(a.gp_re - b.gp_re, a.gp_im - b.gp_im);
    }
    // gc's OMUL of complex numbers: in float64, without the corrections of C's Annex G.
    friend constexpr complex operator*(const complex& a, const complex& b) noexcept {
#if defined(__clang__)
#pragma STDC FP_CONTRACT OFF
#endif
        double ar = a.gp_re, ai = a.gp_im, br = b.gp_re, bi = b.gp_im;
        double re = ar * br - ai * bi;
        double im = ar * bi + ai * br;
        return complex(static_cast<F>(re), static_cast<F>(im));
    }
    // gc's ODIV of complex numbers: a call of runtime.complex128div, through complex128.
    friend complex operator/(const complex& a, const complex& b) noexcept {
        if constexpr (std::is_same_v<F, double>) {
            return complex128div(a, b);
        } else {
            return complex(complex128div(complex128(a), complex128(b)));
        }
    }
    friend constexpr complex operator-(const complex& a) noexcept {
        return complex(-a.gp_re, -a.gp_im);
    }
    friend constexpr bool operator==(const complex& a, const complex& b) noexcept {
        return a.gp_re == b.gp_re && a.gp_im == b.gp_im;
    }

    friend constexpr complex& operator+=(complex& a, const complex& b) noexcept {
        return a = a + b;
    }
    friend constexpr complex& operator-=(complex& a, const complex& b) noexcept {
        return a = a - b;
    }
    friend constexpr complex& operator*=(complex& a, const complex& b) noexcept {
        return a = a * b;
    }
    friend complex& operator/=(complex& a, const complex& b) noexcept { return a = a / b; }

    // c++ and c-- are c += 1 and c -= 1: (1+0i), so the imaginary part gets + 0 and - 0.
    constexpr complex& operator++() noexcept { return *this = *this + complex(F(1)); }
    constexpr complex& operator--() noexcept { return *this = *this - complex(F(1)); }
    constexpr complex operator++(int) noexcept {
        complex old = *this;
        ++*this;
        return old;
    }
    constexpr complex operator--(int) noexcept {
        complex old = *this;
        --*this;
        return old;
    }
};

static_assert(std::is_trivially_copyable_v<complex64> && std::is_standard_layout_v<complex64>);
static_assert(std::is_trivially_copyable_v<complex128> && std::is_standard_layout_v<complex128>);

// real(c) and imag(c): Go's builtins.
template <go_float F>
constexpr F real(const complex<F>& c) noexcept {
    return c.gp_re;
}

template <go_float F>
constexpr F imag(const complex<F>& c) noexcept {
    return c.gp_im;
}

namespace detail {

template <typename T>
inline constexpr bool is_complex = false;
template <go_float F>
inline constexpr bool is_complex<complex<F>> = true;

} // namespace detail

// go_complex: gp::complex64 or gp::complex128.
template <typename T>
concept go_complex = detail::is_complex<std::remove_cv_t<T>>;

} // namespace gp
