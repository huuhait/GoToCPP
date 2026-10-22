// The digits of a float through std::to_chars (ftoa.hpp): the engine that programs use where
// libstdc++ is a shared library. std::to_chars rounds the exact value correctly (ties to
// even), as strconv does.
#include <gocpp/nofortify.h>
#include "ftoa.hpp"

#include <algorithm>
#include <charconv>
#include <cmath>
#include <cstddef>
#include <string>
#include <string_view>

namespace gp::fmt::detail {

namespace {

// value is the float mant * 2^(exp - flt.mantbits), exactly.
double value(uint64_t mant, int exp, const float_info& flt) {
    return std::ldexp(static_cast<double>(mant), exp - flt.mantbits);
}

// push_digit adds a digit to out. The digits beyond what out holds are zeros: a float64 has at
// most 767 significant digits.
void push_digit(float_digits& out, char c) {
    if (out.nd < static_cast<int>(sizeof out.d)) {
        out.d[out.nd++] = c;
    }
}

// scientific_digits reads the digits of "d.ddde+dd" (what std::to_chars wrote) into out: the
// digits without the point, without trailing zeros, and the decimal point after dp digits.
void scientific_digits(float_digits& out, std::string_view text) {
    std::size_t epos = text.find('e');
    out.nd = 0;
    for (char c : text.substr(0, epos)) {
        if (c != '.') {
            push_digit(out, c);
        }
    }
    std::string_view exp_text = text.substr(epos + 1);
    bool exp_negative = exp_text.front() == '-';
    exp_text.remove_prefix(1); // the sign
    int exp = 0;
    std::from_chars(exp_text.data(), exp_text.data() + exp_text.size(), exp);
    if (exp_negative) {
        exp = -exp;
    }
    while (out.nd > 0 && out.d[out.nd - 1] == '0') {
        out.nd--;
    }
    out.dp = out.nd == 0 ? 0 : exp + 1;
}

// scientific appends the float to buf as "d.ddde+dd": the shortest digits for precision < 0
// (a float32 for is32), else precision digits after the point.
std::string scientific(double v, bool is32, int precision) {
    std::string buf(static_cast<std::size_t>(std::max(precision, 0)) + 32, '\0');
    char* first = buf.data();
    char* last = first + buf.size();
    std::to_chars_result r{};
    if (precision < 0) {
        r = is32 ? std::to_chars(first, last, static_cast<float>(v), std::chars_format::scientific)
                 : std::to_chars(first, last, v, std::chars_format::scientific);
    } else {
        r = std::to_chars(first, last, v, std::chars_format::scientific, precision);
    }
    buf.resize(static_cast<std::size_t>(r.ptr - first));
    return buf;
}

} // namespace

void shortest_digits_std(float_digits& out, uint64_t mant, int exp, const float_info& flt) {
    if (mant == 0) {
        out.nd = 0;
        out.dp = 0;
        return;
    }
    double v = value(mant, exp, flt);
    scientific_digits(out, scientific(v, flt.mantbits == float32_info.mantbits, -1));
}

void precision_digits_std(float_digits& out, uint64_t mant, int exp, const float_info& flt,
                          char fmt, int prec) {
    if (mant == 0) {
        out.nd = 0;
        out.dp = 0;
        return;
    }
    double v = value(mant, exp, flt);
    if (fmt != 'f') {
        // 'e', 'E' have prec + 1 digits, 'g', 'G' have prec (at least 1).
        int digits = fmt == 'e' || fmt == 'E' ? prec + 1 : (prec == 0 ? 1 : prec);
        scientific_digits(out, scientific(v, false, digits - 1));
        return;
    }
    // 'f': "ddd.ddd" with prec digits after the point. Its digits are the digits of the number
    // without the point, minus the leading zeros (each of which moves the point) and the trailing
    // zeros.
    std::string buf(static_cast<std::size_t>(330 + prec), '\0');
    std::to_chars_result r =
        std::to_chars(buf.data(), buf.data() + buf.size(), v, std::chars_format::fixed, prec);
    std::string_view text(buf.data(), static_cast<std::size_t>(r.ptr - buf.data()));
    std::size_t point = text.find('.');
    int integer_digits = static_cast<int>(point == std::string_view::npos ? text.size() : point);
    out.nd = 0;
    out.dp = integer_digits;
    for (char c : text) {
        if (c == '.') {
            continue;
        }
        if (out.nd == 0 && c == '0') {
            out.dp--;
            continue;
        }
        push_digit(out, c);
    }
    while (out.nd > 0 && out.d[out.nd - 1] == '0') {
        out.nd--;
    }
    if (out.nd == 0) {
        out.dp = 0;
    }
}

} // namespace gp::fmt::detail
