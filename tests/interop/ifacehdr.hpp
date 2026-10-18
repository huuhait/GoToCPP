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

// Pointers whose pointees are const, and pointers to pointers (iface_cptr_const.gp).
inline Pt g_pt{3, 4};
inline char g_buf[8] = "abc";
inline int g_int = 5;
inline const char* cname() { return "hello"; }
inline char* mname() { return g_buf; }
inline const Pt* cpt() { return &g_pt; }
inline Pt* mpt() { return &g_pt; }
inline const int* cint() { return &g_int; }
inline int* mint() { return &g_int; }
inline Pt** ppt() {
    static Pt* p = &g_pt;
    return &p;
}
inline const Pt* const* cppt() { return ppt(); }
} // namespace ifh
