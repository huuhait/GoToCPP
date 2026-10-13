// A small C++ helper next to the .gp file: gocpp includes it and compiles
// mathx.cpp (if any) in this folder, like cgo does for .c files.
#pragma once
#include <cmath>

static inline double hypot3(double a, double b, double c) {
	return std::sqrt(a * a + b * b + c * c);
}
