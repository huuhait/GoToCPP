#pragma once

// A header-only library: nothing to compile or link.
namespace hdr {

inline int clamp(int v, int lo, int hi) { return v < lo ? lo : v > hi ? hi : v; }

inline int sum3(int a, int b, int c) { return a + b + c; }

constexpr int VERSION_MAJOR = 1;

} // namespace hdr
