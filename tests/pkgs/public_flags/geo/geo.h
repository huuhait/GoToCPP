#ifndef GEO_SCALE
#error GEO_SCALE must be defined by the flags of package geo (public flags)
#endif
#pragma once
typedef struct { int v[GEO_SCALE]; } geo_pt;
static inline int geo_sum(const geo_pt* p) {
    int s = 0;
    for (int i = 0; i < GEO_SCALE; i++) s += p->v[i];
    return s;
}
static inline void geo_fill(geo_pt* p, int x) {
    for (int i = 0; i < GEO_SCALE; i++) p->v[i] = x + i;
}
