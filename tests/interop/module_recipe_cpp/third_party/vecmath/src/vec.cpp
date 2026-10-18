#ifndef VECMATH_BUILDING
#error "the private define of the recipe is missing"
#endif
#include <vecmath/vec.hpp>

#if __cplusplus < 201703L
#error "the library is C++17 or newer"
#endif

int vec_lib_size() { return (int)sizeof(Vec3); }

int vec_dot3(int ax, int ay, int az, int bx, int by, int bz) {
	Vec3 a{(double)ax, (double)ay, (double)az, 0}, b{(double)bx, (double)by, (double)bz, 0};
	return (int)(vecmath_detail::mul(a.x, b.x) + vecmath_detail::mul(a.y, b.y) + vecmath_detail::mul(a.z, b.z));
}

int vec_cxx17() {
	// if constexpr is C++17
	if constexpr (sizeof(Vec3) == 32) {
		return 1;
	} else {
		return 0;
	}
}
