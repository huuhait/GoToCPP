// A helper header for the tests of composite literals (step 4.6): C structs, also nested, with
// an array field, a tag name, bit-fields, pointers and small integer fields. It is C, so gocpp wraps it in
// extern "C".
#pragma once

typedef struct {
    int x, y;
} Vec;

typedef struct {
    Vec min, max;
} Box;

typedef struct {
    unsigned char r, g, b, a;
} Color;

typedef struct {
    const char* name;
    Vec* at;
    double weight;
    Color color;
} Item;

typedef struct {
    int v[3];
    int n;
} Triple;

struct pair_s {
    int first, second;
};

typedef struct {
    unsigned a : 1;
    unsigned b : 3;
    unsigned c : 4;
} Flags;

static inline int vec_sum(Vec v) {
    return v.x + v.y;
}
static inline int box_area(Box b) {
    return (b.max.x - b.min.x) * (b.max.y - b.min.y);
}
static inline int pair_diff(struct pair_s p) {
    return p.first - p.second;
}

static inline int flags_digits(Flags f) {
    return f.a * 100 + f.b * 10 + f.c;
}

static inline int triple_sum(Triple t) {
    int s = 0;
    for (int i = 0; i < t.n; i++) {
        s += t.v[i];
    }
    return s;
}
