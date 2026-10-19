// The native functions of package math that are not one instruction (gocpp/math.hpp, Phase 09 step
// 9.5): exp and log, a transcription of Go's amd64 assembly (src/math/exp_amd64.s and log_amd64.s
// of Go 1.25), operation by operation, so that the results are Go's, bit for bit, also where the
// pure-Go versions differ. Compiled with -ffp-contract=off (the driver's flags): every a*b+c below
// is two roundings, and the fused ones are the explicit fused operations. Standard C++ only.
#include <gocpp/rt.hpp>

#include <cmath>
#include <cstdlib>
#include <cstring>
#include <limits>

// exp_fma uses the FMA instruction inline (the x86-64 baseline has none, so the function is
// compiled for it; it must only run on a CPU that has it: exp() checks, the unit test checks).
#if defined(__x86_64__)
#define GP_TARGET_FMA __attribute__((target("fma")))
#else
#define GP_TARGET_FMA
#endif

namespace gp::math {

// Go's x86HasFMA. A plain global that the inline fma reads; set before main.
const bool has_fma = [] {
#if defined(__x86_64__)
    __builtin_cpu_init();
    return static_cast<bool>(__builtin_cpu_supports("fma"));
#else
    return false;
#endif
}();

const bool has_sse41 = [] {
#if defined(__x86_64__)
    __builtin_cpu_init();
    return static_cast<bool>(__builtin_cpu_supports("sse4.1"));
#else
    return false;
#endif
}();

namespace {

constexpr uint64_t pos_inf = 0x7FF0000000000000;
constexpr uint64_t neg_inf = 0xFFF0000000000000;
// The NaN of Go's assembly (not the one of the hardware, which has the sign bit set).
constexpr uint64_t go_nan = 0x7FF8000000000001;
constexpr uint64_t sign_mask = uint64_t(1) << 63;

// useFMA is Go's: cpu.X86.HasAVX && cpu.X86.HasFMA.
bool use_fma() noexcept {
    static const bool v = [] {
#if defined(__x86_64__) || defined(__i386__)
        __builtin_cpu_init();
        return __builtin_cpu_supports("avx") && __builtin_cpu_supports("fma") &&
               !cpu_feature_disabled(std::getenv("GODEBUG"), "avx") &&
               !cpu_feature_disabled(std::getenv("GODEBUG"), "fma");
#else
        return false;
#endif
    }();
    return v;
}

// CVTSD2SL: round to nearest even (the default rounding mode), and the "integer indefinite" value
// INT32_MIN when the result does not fit.
int32_t cvtsd2sl(double x) noexcept {
#if defined(__x86_64__)
    int32_t r;
    asm("cvtsd2si %1, %0" : "=r"(r) : "x"(x));
    return r;
#else
    const double r = std::nearbyint(x);
    if (r >= -2147483648.0 && r <= 2147483647.0) {
        return static_cast<int32_t>(r);
    }
    return std::numeric_limits<int32_t>::min();
#endif
}

constexpr double LOG2E = 1.4426950408889634073599246810018920;
constexpr double LN2U = 0.69314718055966295651160180568695068359375;
constexpr double LN2L = 0.28235290563031577122588448175013436025525412068e-12;
constexpr double Overflow = 7.09782712893384e+02;

// exprodata of the assembly: 0.5, 1.0, 2.0 and the Taylor coefficients.
constexpr double C0 = 0.5;
constexpr double C8 = 1.0;
constexpr double C16 = 2.0;
constexpr double C24 = 1.6666666666666666667e-1;
constexpr double C32 = 4.1666666666666666667e-2;
constexpr double C40 = 8.3333333333333333333e-3;
constexpr double C48 = 1.3888888888888888889e-3;
constexpr double C56 = 1.9841269841269841270e-4;
constexpr double C64 = 2.4801587301587301587e-5;

// The label ldexp of the assembly: fr * 2**exponent, with its denormal and overflow exits.
double ldexp_tail(double x0, int32_t bx) noexcept {
    bx = static_cast<int32_t>(static_cast<uint32_t>(bx) + 0x3FFu);
    if (bx <= 0) {
        // denormal
        if (bx < -52) {
            return 0; // underflow
        }
        bx = static_cast<int32_t>(static_cast<uint32_t>(bx) + 0x3FEu);
        x0 *= std::bit_cast<double>(static_cast<uint64_t>(static_cast<uint32_t>(bx)) << 52);
        bx = 1;
    } else if (bx >= 0x7FF) {
        return std::bit_cast<double>(pos_inf); // overflow
    }
    return x0 * std::bit_cast<double>(static_cast<uint64_t>(static_cast<uint32_t>(bx)) << 52);
}

} // namespace

bool cpu_feature_disabled(const char* godebug, const char* name) {
    if (godebug == nullptr) {
        return false;
    }
    bool off = false;
    const std::size_t n = std::strlen(name);
    // Go applies the settings in order: for a name, the last one wins ("all" is every feature).
    for (const char* p = godebug; *p != '\0';) {
        const char* end = std::strchr(p, ',');
        const std::size_t len = end != nullptr ? static_cast<std::size_t>(end - p) : std::strlen(p);
        if (len > 4 && std::strncmp(p, "cpu.", 4) == 0) {
            const char* key = p + 4;
            const char* eq = static_cast<const char*>(std::memchr(key, '=', len - 4));
            if (eq != nullptr) {
                const std::size_t klen = static_cast<std::size_t>(eq - key);
                const std::size_t vlen = len - 4 - klen - 1;
                const bool is_off = vlen == 3 && std::strncmp(eq + 1, "off", 3) == 0;
                const bool is_on = vlen == 2 && std::strncmp(eq + 1, "on", 2) == 0;
                const bool named = klen == n && std::strncmp(key, name, n) == 0;
                const bool all = klen == 3 && std::strncmp(key, "all", 3) == 0;
                if (named || all) {
                    if (is_off) {
                        off = true;
                    } else if (is_on) {
                        off = false;
                    }
                }
            }
        }
        if (end == nullptr) {
            break;
        }
        p = end + 1;
    }
    return off;
}

double exp_nofma(double x) {
    const uint64_t bits = std::bit_cast<uint64_t>(x);
    if ((bits & ~sign_mask) >= pos_inf) { // not finite
        if (bits == neg_inf) {
            return 0;
        }
        return x; // NaN or +Inf
    }
    if (x > Overflow) {
        return std::bit_cast<double>(pos_inf);
    }
    const int32_t bx = cvtsd2sl(LOG2E * x);
    const double k = static_cast<double>(bx);
    double x0 = x;
    x0 -= LN2U * k;
    x0 -= LN2L * k;
    x0 *= 0.0625; // reduce argument
    // Taylor series evaluation
    double p = C64;
    p *= x0;
    p += C56;
    p *= x0;
    p += C48;
    p *= x0;
    p += C40;
    p *= x0;
    p += C32;
    p *= x0;
    p += C24;
    p *= x0;
    p += C0;
    p *= x0;
    p += C8;
    x0 *= p;
    for (int i = 0; i < 4; i++) {
        const double t = C16 + x0;
        x0 *= t;
    }
    x0 += C8;
    return ldexp_tail(x0, bx);
}

GP_TARGET_FMA double exp_fma(double x) {
    const uint64_t bits = std::bit_cast<uint64_t>(x);
    if ((bits & ~sign_mask) >= pos_inf) {
        if (bits == neg_inf) {
            return 0;
        }
        return x;
    }
    if (x > Overflow) {
        return std::bit_cast<double>(pos_inf);
    }
    const int32_t bx = cvtsd2sl(LOG2E * x);
    const double k = static_cast<double>(bx);
    double x0 = x;
    x0 = __builtin_fma(-k, LN2U, x0);
    x0 = __builtin_fma(-k, LN2L, x0);
    x0 *= 0.0625;
    double p = C64;
    p = __builtin_fma(x0, p, C56);
    p = __builtin_fma(x0, p, C48);
    p = __builtin_fma(x0, p, C40);
    p = __builtin_fma(x0, p, C32);
    p = __builtin_fma(x0, p, C24);
    p = __builtin_fma(x0, p, C0);
    p = __builtin_fma(x0, p, C8);
    x0 *= p;
    double t = x0 + C16;
    x0 *= t;
    t = x0 + C16;
    x0 *= t;
    t = x0 + C16;
    x0 *= t;
    t = x0 + C16;
    x0 = __builtin_fma(t, x0, C8);
    return ldexp_tail(x0, bx);
}

double exp(double x) {
    return use_fma() ? exp_fma(x) : exp_nofma(x);
}

double log(double x) {
    constexpr double HSqrt2 = 7.07106781186547524401e-01;
    constexpr double Ln2Hi = 6.93147180369123816490e-01;
    constexpr double Ln2Lo = 1.90821492927058770002e-10;
    constexpr double L1 = 6.666666666666735130e-01;
    constexpr double L2 = 3.999999999940941908e-01;
    constexpr double L3 = 2.857142874366239149e-01;
    constexpr double L4 = 2.222219843214978396e-01;
    constexpr double L5 = 1.818357216161805012e-01;
    constexpr double L6 = 1.531383769920937332e-01;
    constexpr double L7 = 1.479819860511658591e-01;

    const uint64_t bits = std::bit_cast<uint64_t>(x);
    if ((bits & ~sign_mask) == 0) {
        return std::bit_cast<double>(neg_inf);
    }
    if (static_cast<int64_t>(bits) < 0) {
        return std::bit_cast<double>(go_nan);
    }
    if (pos_inf <= bits) {
        return x; // +Inf or NaN
    }
    // f1, ki := math.Frexp(x); k := float64(ki). For a subnormal x the exponent field is 0 here, as
    // in Go's assembly (Go's bug: the result is wrong for subnormal numbers).
    double f1 = std::bit_cast<double>((bits & 0x000FFFFFFFFFFFFF) | 0x3FE0000000000000);
    double k = static_cast<double>(static_cast<int32_t>((bits >> 52) & 0x7FF) - 0x3FE);
    // if f1 < math.Sqrt2/2 { k -= 1; f1 *= 2 }  (the assembly's compare is <=)
    const double adj = HSqrt2 >= f1 ? 1.0 : 0.0;
    k -= adj;
    f1 *= adj + 1.0;
    const double f = f1 - 1.0;
    const double s = f / (2.0 + f);
    const double s2 = s * s;
    const double s4 = s2 * s2;
    double t1 = L7 * s4;
    t1 += L5;
    t1 *= s4;
    t1 += L3;
    t1 *= s4;
    t1 += L1;
    t1 = s2 * t1;
    double t2 = L6 * s4;
    t2 += L4;
    t2 *= s4;
    t2 += L2;
    t2 = s4 * t2;
    const double R = t1 + t2;
    double hfsq = 0.5 * f;
    hfsq *= f;
    const double a = s * (R + hfsq);
    const double b = a + Ln2Lo * k;
    double r = hfsq - b;
    r -= f;
    return k * Ln2Hi - r;
}

} // namespace gp::math
