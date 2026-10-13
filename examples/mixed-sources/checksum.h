/* checksum.h: C functions, defined in checksum.c. The header has no
 * __cplusplus guard on purpose: gocpp sees that it parses as C and includes
 * it inside extern "C" { ... } (Phase 04 §2.9). */
#ifndef CHECKSUM_H
#define CHECKSUM_H

#include <stddef.h>
#include <stdint.h>

#define CHECKSUM_VERSION 2

/* fnv1a32 returns the 32-bit FNV-1a hash of the n bytes at data. */
uint32_t fnv1a32(const char* data, size_t n);

/* span_stats summarizes an array of ints. */
typedef struct {
    int min, max;
    long long sum;
} span_stats;

/* stats_of returns the smallest, the largest and the sum of n ints. */
span_stats stats_of(const int* xs, size_t n);

#endif
