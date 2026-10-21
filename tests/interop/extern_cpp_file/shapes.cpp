// C++ functions that no header declares: a function of a namespace, two overloads, a function
// with C++ linkage whose parameter is a pointer to const, and one with C linkage.
#include <cstddef>
#include <cstdint>

int total(const int *values, int n) {
	int sum = 0;
	for (int i = 0; i < n; i++) {
		sum += values[i];
	}
	return sum;
}

namespace shapes {
double area(double width, double height) { return width * height; }

int count(int n) { return n; }

int count(double x) { return 100 + static_cast<int>(x); }

namespace inner {
const char *name() { return "inner"; }
} // namespace inner
} // namespace shapes

extern "C" bool shapes_fill(void *buffer, size_t n, int32_t *out) {
	unsigned char *bytes = static_cast<unsigned char *>(buffer);
	for (size_t i = 0; i < n; i++) {
		bytes[i] = static_cast<unsigned char>(i + 1);
	}
	*out = 42;
	return true;
}
