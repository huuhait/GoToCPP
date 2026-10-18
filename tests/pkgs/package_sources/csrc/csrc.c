#include "csrc.h"
#include <math.h>
#ifndef CSRC_BASE
#error the flags of package csrc are missing in its own C file
#endif
int csrc_twice(int x) { return 2 * x + CSRC_BASE; }
double csrc_root(double x) { return sqrt(x); }
