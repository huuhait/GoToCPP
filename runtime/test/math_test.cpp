// The unit tests of the native functions of package math (gocpp/math.hpp, Phase 09 step 9.5): exp
// and log (Go's amd64 assembly, with both paths of exp), fma, and the exact functions, against the
// results of real Go on amd64 (math_data.hpp, written by tools/mathcorpus/cxxdata.py).
// runtime/cxx_test.go compiles this file with the runtime's .cpp files, with g++ and clang++, at
// -O0, at -O2 and with ASan + UBSan, and runs it like test/unit.cpp. It prints "ok: N checks" and
// exits 0, or prints each failed check and exits 1.
#include <gocpp/rt.hpp>
#include <gocpp/os.hpp>

#include <bit>
#include <cmath>
#include <cstdint>
#include <limits>
#include <cstdlib>
#include <string>
#include <vector>

#include "math_data.hpp"

namespace {

int checks = 0;
int failures = 0;

void report(bool ok, const char* file, int line, const std::string& what) {
    checks++;
    if (!ok) {
        failures++;
        std::string msg = std::string(file) + ":" + std::to_string(line) + ": " + what + "\n";
        gp::os::write(gp::os::stderr_fd, msg.data(), msg.size());
    }
}

#define CHECK(...) report((__VA_ARGS__), __FILE__, __LINE__, "CHECK(" #__VA_ARGS__ ") failed")

std::string hex(uint64_t v) {
    char buf[32];
    std::snprintf(buf, sizeof buf, "%016llx", static_cast<unsigned long long>(v));
    return buf;
}

double d(uint64_t bits) {
    return std::bit_cast<double>(bits);
}

uint64_t b(double x) {
    return std::bit_cast<uint64_t>(x);
}

#define CHECK_BITS(got, want, what)                                                                \
    do {                                                                                           \
        const uint64_t gp_got = (got);                                                             \
        const uint64_t gp_want = (want);                                                           \
        report(gp_got == gp_want, __FILE__, __LINE__,                                              \
               std::string(what) + ": got " + hex(gp_got) + ", want " + hex(gp_want));             \
    } while (0)

void test_exp() {
    // exp_fma is compiled for the FMA instruction: it runs only on a CPU that has it
    if (gp::math::has_fma) {
        for (const F1& c : exp_fma_cases) {
            CHECK_BITS(b(gp::math::exp_fma(d(c.x))), c.want, "exp_fma(" + hex(c.x) + ")");
        }
    }
    for (const F1& c : exp_nofma_cases) {
        CHECK_BITS(b(gp::math::exp_nofma(d(c.x))), c.want, "exp_nofma(" + hex(c.x) + ")");
    }
    // exp picks one of the two paths, by the CPU and GODEBUG, as Go does: the table of the path it
    // took
    const bool fma_path = gp::math::has_fma &&
                          !gp::math::cpu_feature_disabled(std::getenv("GODEBUG"), "fma") &&
                          !gp::math::cpu_feature_disabled(std::getenv("GODEBUG"), "avx");
    const F1* table = fma_path ? exp_fma_cases : exp_nofma_cases;
    for (std::size_t i = 0; i < sizeof exp_fma_cases / sizeof exp_fma_cases[0]; i++) {
        CHECK_BITS(b(gp::math::exp(d(table[i].x))), table[i].want, "exp(" + hex(table[i].x) + ")");
    }
}

void test_log() {
    for (const F1& c : log_cases) {
        CHECK_BITS(b(gp::math::log(d(c.x))), c.want, "log(" + hex(c.x) + ")");
    }
    // the quirk: Go's amd64 Log of a subnormal number is not the logarithm
    CHECK_BITS(b(gp::math::log(5e-324)), b(-709.0895657128241), "log(5e-324)");
    CHECK(gp::math::log(5e-324) == -709.0895657128241);
}

void test_fma() {
    for (const F3& c : fma_cases) {
        CHECK_BITS(b(gp::math::fma(d(c.x), d(c.y), d(c.z))), c.want,
                   "fma(" + hex(c.x) + ", " + hex(c.y) + ", " + hex(c.z) + ")");
    }
    // one rounding: (1+2^-27)^2 - (1+2^-26) is 2^-54 fused, 0 in two steps
    const double x = 1 + 0x1p-27;
    CHECK(gp::math::fma(x, x, -(1 + 0x1p-26)) == 0x1p-54);
    volatile double vx = x;
    const double prod = vx * vx;
    CHECK(prod + -(1 + 0x1p-26) == 0); // not contracted by the compiler (-ffp-contract=off)
}

void test_exact() {
    CHECK(gp::math::sqrt(2) == 1.4142135623730951);
    CHECK(b(gp::math::sqrt(-1)) == 0xfff8000000000000ULL);
    CHECK(std::isinf(gp::math::sqrt(std::numeric_limits<double>::infinity())));
    CHECK(b(gp::math::sqrt(-0.0)) == b(-0.0));
    CHECK(gp::math::floor(-1.5) == -2 && gp::math::ceil(-1.5) == -1 && gp::math::trunc(-1.5) == -1);
    CHECK(b(gp::math::ceil(-0.5)) == b(-0.0));
    CHECK(b(gp::math::floor(-0.0)) == b(-0.0));
    CHECK(gp::math::round_to_even(2.5) == 2 && gp::math::round_to_even(3.5) == 4 &&
          gp::math::round_to_even(-2.5) == -2);
    CHECK(gp::math::abs(-3) == 3);
    CHECK(b(gp::math::abs(d(0xfff8000000000001ULL))) == 0x7ff8000000000001ULL);
    CHECK(gp::math::float64bits(1.5) == 0x3ff8000000000000ULL);
    CHECK(gp::math::float64frombits(0x4000000000000000ULL) == 2.0);
    CHECK(gp::math::float32bits(1.5f) == 0x3fc00000u);
    CHECK(gp::math::float32frombits(0x40000000u) == 2.0f);
    // a signaling NaN keeps its payload through the casts, and floor and friends make it quiet
    CHECK(gp::math::float64bits(gp::math::float64frombits(0x7ff0000000000001ULL)) ==
          0x7ff0000000000001ULL);
    CHECK(gp::math::float32bits(gp::math::float32frombits(0x7f800001u)) == 0x7f800001u);
    CHECK(b(gp::math::round_to_even(d(0x7ff4479c6aacc07aULL))) == 0x7ffc479c6aacc07aULL);
}

// The SSE2 versions of floor, ceil, trunc and round_to_even (what a CPU without SSE4.1 runs) give
// the bits of the C library's functions, and a quiet NaN with the payload of a signaling one.
void test_sse2_rounding() {
    std::vector<double> xs = {0.0,
                              -0.0,
                              0.5,
                              -0.5,
                              1.5,
                              -1.5,
                              2.5,
                              -2.5,
                              3.5,
                              0.49999999999999994,
                              0.5000000000000001,
                              4503599627370495.5,
                              4503599627370496.0,
                              4503599627370497.0,
                              9007199254740993.0,
                              1e300,
                              -1e300,
                              std::numeric_limits<double>::infinity(),
                              -std::numeric_limits<double>::infinity(),
                              5e-324,
                              -5e-324,
                              2.2250738585072014e-308,
                              0.9999999999999999,
                              -0.9999999999999999,
                              1e15 + 0.5,
                              -(1e15 + 0.5)};
    for (int i = -40; i <= 40; i++) {
        xs.push_back(i / 4.0);
        xs.push_back(i / 4.0 + 1e-13);
    }
    uint64_t state = 88172645463325252ULL;
    for (int i = 0; i < 4000; i++) {
        state ^= state << 13;
        state ^= state >> 7;
        state ^= state << 17;
        xs.push_back(d(state));
        xs.push_back(static_cast<double>(static_cast<int64_t>(state >> 20)) / 1024.0);
    }
    for (double x : xs) {
        if (x != x) {
            continue;
        }
        const std::string w = hex(b(x));
        CHECK_BITS(b(gp::math::detail::floor_sse2(x)), b(std::floor(x)), "floor_sse2(" + w + ")");
        CHECK_BITS(b(gp::math::detail::ceil_sse2(x)), b(std::ceil(x)), "ceil_sse2(" + w + ")");
        CHECK_BITS(b(gp::math::detail::trunc_sse2(x)), b(std::trunc(x)), "trunc_sse2(" + w + ")");
        CHECK_BITS(b(gp::math::detail::round_to_even_sse2(x)), b(std::nearbyint(x)),
                   "round_to_even_sse2(" + w + ")");
        // the public functions are the same function on every CPU
        CHECK_BITS(b(gp::math::floor(x)), b(std::floor(x)), "floor(" + w + ")");
        CHECK_BITS(b(gp::math::round_to_even(x)), b(std::nearbyint(x)), "round_to_even(" + w + ")");
    }
    for (uint64_t nan : {0x7ff0000000000001ULL, 0xfff4479c6aacc07aULL, 0x7ff8000000000000ULL,
                         0xffffffffffffffffULL}) {
        const uint64_t quiet = nan | 0x0008000000000000ULL;
        CHECK_BITS(b(gp::math::detail::floor_sse2(d(nan))), quiet, "floor_sse2(NaN)");
        CHECK_BITS(b(gp::math::detail::ceil_sse2(d(nan))), quiet, "ceil_sse2(NaN)");
        CHECK_BITS(b(gp::math::detail::trunc_sse2(d(nan))), quiet, "trunc_sse2(NaN)");
        CHECK_BITS(b(gp::math::detail::round_to_even_sse2(d(nan))), quiet,
                   "round_to_even_sse2(NaN)");
        CHECK_BITS(b(gp::math::round_to_even(d(nan))), quiet, "round_to_even(NaN)");
    }
}

void test_cpu_options() {
    using gp::math::cpu_feature_disabled;
    CHECK(!cpu_feature_disabled(nullptr, "fma"));
    CHECK(!cpu_feature_disabled("", "fma"));
    CHECK(cpu_feature_disabled("cpu.fma=off", "fma"));
    CHECK(!cpu_feature_disabled("cpu.fma=off", "avx"));
    CHECK(cpu_feature_disabled("foo=1,cpu.avx=off,bar=2", "avx"));
    CHECK(cpu_feature_disabled("cpu.all=off", "fma"));
    CHECK(!cpu_feature_disabled("cpu.fma=on", "fma"));
    CHECK(!cpu_feature_disabled("cpu.fma=off,cpu.fma=on", "fma"));
    CHECK(cpu_feature_disabled("cpu.fma=on,cpu.fma=off", "fma"));
    CHECK(!cpu_feature_disabled("cpu.fmaa=off", "fma"));
    CHECK(!cpu_feature_disabled("xcpu.fma=off", "fma"));
    CHECK(!cpu_feature_disabled("cpu.fma=offf", "fma"));
    CHECK(!cpu_feature_disabled("cpu.fma", "fma"));
}

} // namespace

int main() {
    test_exp();
    test_log();
    test_fma();
    test_exact();
    test_sse2_rounding();
    test_cpu_options();
    if (failures != 0) {
        return 1;
    }
    std::string msg = "ok: " + std::to_string(checks) + " checks\n";
    gp::os::write(gp::os::stdout_fd, msg.data(), msg.size());
    return 0;
}
