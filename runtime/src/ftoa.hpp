// The digits of a float (internal to the runtime: src/ftoa.cpp and src/ftoa_std.cpp, used by
// src/fmt.cpp). Two engines that give the same digits for the same float, byte for byte:
//
//   - the runtime's own (ftoa.cpp), a port of the digit generation of Go 1.25's strconv (ftoa.go,
//     ftoaryu.go, decimal.go): the Ryu algorithm for the shortest digits that read back as the
//     same float and for up to 18 digits, and multiprecision decimal arithmetic for the rest. It
//     costs 25 KB;
//   - std::to_chars (ftoa_std.cpp). libstdc++'s copy is 190 KB (its Ryu tables alone are 110 KB),
//     which a program that links libstdc++ statically carries whole: MinGW always does. Where
//     libstdc++ is a shared library (Linux) or the system's libc++ (macOS) it costs nothing.
//
// own_ftoa says which one fmt.cpp uses: ours on Windows, std::to_chars elsewhere (override with
// -DGOCPP_OWN_FTOA=0 or 1). The unit tests (test/fmt_test.cpp) call both engines and compare
// them, so each is tested on every platform.
#pragma once

#include <cstdint>

#ifndef GOCPP_OWN_FTOA
#ifdef _WIN32
#define GOCPP_OWN_FTOA 1
#else
#define GOCPP_OWN_FTOA 0
#endif
#endif

namespace gp::fmt::detail {

inline constexpr bool own_ftoa = GOCPP_OWN_FTOA != 0;

// float_info describes a floating-point format, as strconv's floatInfo.
struct float_info {
    int mantbits;
    int expbits;
    int bias;
};

inline constexpr float_info float32_info{23, 8, -127};
inline constexpr float_info float64_info{52, 11, -1023};

// float_digits holds the decimal digits of a positive float: d[0:nd] with the decimal point after
// dp digits (the value is 0.d * 10^dp), as strconv's decimalSlice. Zero has no digits and dp 0.
// There are no trailing zeros. 800 digits hold every float64 exactly (at most 767 are needed).
struct float_digits {
    char d[800];
    int nd = 0;
    int dp = 0;
};

// The value of the float is mant * 2^(exp - flt.mantbits): mant has its implicit top bit, and
// exp is the exponent with the bias removed (1 + bias for a denormal), as strconv's genericFtoa
// computes them.

// shortest_digits writes the shortest digits that read back as the same float (a float32 when
// flt is float32_info).
void shortest_digits(float_digits& out, uint64_t mant, int exp, const float_info& flt);

// fixed_digits writes the float rounded to the given number of digits (ties to even, on the
// exact value) when the Ryu code covers it: digits <= 9 for a float32, <= 18 otherwise. It
// returns false, and writes nothing, for more digits: exact_digits does those.
bool fixed_digits(float_digits& out, uint64_t mant, int exp, const float_info& flt, int digits);

// exact_digits writes the exact value rounded as the verb fmt asks, with multiprecision decimal
// arithmetic: 'e'/'E' round to prec + 1 digits, 'f' to prec digits after the decimal point,
// 'g'/'G' to prec digits (at least 1).
void exact_digits(float_digits& out, uint64_t mant, int exp, const float_info& flt, char fmt,
                  int prec);

// The same digits through std::to_chars (ftoa_std.cpp): shortest_digits_std as shortest_digits,
// and precision_digits_std for the cases of fixed_digits and exact_digits together: the float
// rounded as the verb fmt asks, for any prec.
void shortest_digits_std(float_digits& out, uint64_t mant, int exp, const float_info& flt);
void precision_digits_std(float_digits& out, uint64_t mant, int exp, const float_info& flt,
                          char fmt, int prec);

} // namespace gp::fmt::detail
