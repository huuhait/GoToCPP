// A header for the interface tests (iface_cpp_pointers.gp, Phase 05 step 5.4): a C++ class with
// methods, == and no std::hash, a plain struct, and one without ==.
#pragma once

namespace ifh {
struct Counter {
    int n = 0;
    Counter() = default;
    explicit Counter(int k) : n(k) {}
    int inc() { return ++n; }
    int get() const { return n; }
    bool operator==(const Counter& o) const { return n == o.n; }
};
struct Pt {
    int x, y;
};
struct NoEq {
    int z = 0;
};
inline Counter make(int k) { return Counter(k); }
inline Pt make_pt(int x, int y) { return Pt{x, y}; }
inline NoEq make_noeq(int z) {
    NoEq e;
    e.z = z;
    return e;
}
} // namespace ifh
