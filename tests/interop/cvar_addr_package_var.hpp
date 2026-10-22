// A header for cvar_addr_package_var.gp: a C++ class whose objects are counted while they live, and
// a plain struct.
#pragma once

namespace cav {
inline int live = 0;
struct Tracked {
    int n = 0;
    Tracked() { live++; }
    explicit Tracked(int k) : n(k) { live++; }
    Tracked(const Tracked& o) : n(o.n) { live++; }
    ~Tracked() { live--; }
    int inc() { return ++n; }
};
struct Pair {
    int a;
    int b;
};
struct Inner {
    int a;
    int b;
};
struct Rec {
    Inner in;
    int arr[3];
};
inline int alive() {
    return live;
}
} // namespace cav
