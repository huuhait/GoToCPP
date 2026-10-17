// A helper header for helper_header.gp (README §2.10): a header next to the .gp file is a package
// (mathx). Everything is inline or a template, so no .cpp file is needed. Another local header
// (units.h) is found next to this one.
#pragma once
#include <cmath>
#include <string>
#include "units.h"

static inline double hypot3(double a, double b, double c) { return std::sqrt(a * a + b * b + c * c); }

template <typename T>
T clamp_to(T v, T lo, T hi) { return v < lo ? lo : (v > hi ? hi : v); }

constexpr int mx_limit = 100;

struct mx_counter {
    int n = 0;
    void add(int d) { n += d; }
    int get() const { return n; }
};

inline std::string mx_describe(double meters) {
    return std::to_string(static_cast<int>(meters * MX_CM_PER_M)) + " cm";
}

enum mx_mode { MX_FAST, MX_SLOW = 5 };
