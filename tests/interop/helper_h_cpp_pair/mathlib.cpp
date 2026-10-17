#include "mathlib.h"

// Defined with C++ linkage: the calls from the generated code are C++ calls, too.
int add(int a, int b) { return a + b; }
int twice(int a) { return add(a, a); }
