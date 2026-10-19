// C code without errors, for clean_c.gp: allocates, shifts by less than the width, frees.
#pragma once

#include <stdlib.h>

static inline int sum_block(int n) {
    int* p = (int*)malloc(sizeof(int) * (size_t)n);
    int s = 0;
    for (int i = 0; i < n; i++) {
        p[i] = 1 << (i % 8);
        s += p[i];
    }
    free(p);
    return s;
}
