// C code with a heap buffer overflow, for canary_asan.gp: the write is one int past the end of
// the block. AddressSanitizer must report it.
#pragma once

#include <stdlib.h>

static inline int overflow_heap(int n) {
    volatile int* p = (volatile int*)malloc(sizeof(int) * (size_t)n);
    p[n] = 7; // one past the end
    int v = p[n];
    free((void*)p);
    return v;
}
