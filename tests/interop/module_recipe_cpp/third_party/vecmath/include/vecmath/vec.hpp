#pragma once
#include <vecmath/detail.h>

#ifdef VECMATH_WIDE
struct Vec3 { double x, y, z, w; };
#else
struct Vec3 { double x, y, z; };
#endif

inline int vec_header_size() { return (int)sizeof(Vec3); }
int vec_lib_size();
int vec_dot3(int ax, int ay, int az, int bx, int by, int bz);
int vec_os_code(); // 1 Linux, 2 Windows, 3 macOS
int vec_cxx17();
