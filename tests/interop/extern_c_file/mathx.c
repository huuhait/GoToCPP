// Defined here, declared by no header: main.gp declares them with //gocpp:extern.
#include <stdint.h>
#include <string.h>

int mathx_add3(int a, int b, int c) { return a + b + c; }

double mathx_scale(double x, double factor) { return x * factor; }

size_t mathx_length(const char *text) { return strlen(text); }

// Writes the quotient and the remainder through pointers, returns 0, or 1 for a zero divisor.
int mathx_divmod(int a, int b, int32_t *quotient, int32_t *remainder) {
	if (b == 0) {
		return 1;
	}
	*quotient = a / b;
	*remainder = a % b;
	return 0;
}

// Fills n bytes with 1, 2, 3, ...
void mathx_fill(void *buffer, size_t n) {
	unsigned char *bytes = buffer;
	for (size_t i = 0; i < n; i++) {
		bytes[i] = (unsigned char)(i + 1);
	}
}

_Bool mathx_even(unsigned char n) { return n % 2 == 0; }
