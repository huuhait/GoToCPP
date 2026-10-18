/* Bit-fields of every kind for c_bitfields.gp (step 4.18): unsigned, signed, a flag, an enum and a
   field wider than 32 bits. Plain C, so gocpp wraps it in extern "C". */
#ifndef CBITFIELDS_H
#define CBITFIELDS_H

#include <stdbool.h>

enum shade { DARK, MID = 2, LIGHT };

typedef struct {
    unsigned u3 : 3;
    int s5 : 5;
    bool flag : 1;
    enum shade tone : 2;
    long long wide : 40;
    unsigned char pad;
} BF;

static inline BF bf_make(unsigned u, int s, int f, long long w) {
    BF b = {0};
    b.u3 = u;
    b.s5 = s;
    b.flag = f;
    b.tone = LIGHT;
    b.wide = w;
    b.pad = 9;
    return b;
}

#endif
