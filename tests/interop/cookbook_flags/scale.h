#pragma once

#include <math.h>

#ifndef SCALE
#define SCALE 1
#endif

// scaled uses a macro that the package's //gocpp:cflags line defines, and libm.
static inline double scaled(double x) {
    return cbrt(x) * SCALE;
}
