/* checksum.c: compiled with the C compiler ($CC -std=c17) and linked in. */
#include "checksum.h"

uint32_t fnv1a32(const char* data, size_t n) {
    uint32_t h = 2166136261u;
    for (size_t i = 0; i < n; i++) {
        h ^= (unsigned char)data[i];
        h *= 16777619u;
    }
    return h;
}

span_stats stats_of(const int* xs, size_t n) {
    span_stats s = {0, 0, 0};
    for (size_t i = 0; i < n; i++) {
        if (i == 0 || xs[i] < s.min) {
            s.min = xs[i];
        }
        if (i == 0 || xs[i] > s.max) {
            s.max = xs[i];
        }
        s.sum += xs[i];
    }
    return s;
}
