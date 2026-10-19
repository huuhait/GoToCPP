// Types for designated_order.gp and its C++ twin.
#pragma once

struct Inner {
    int a;
    int b;
};

struct Rect {
    int x, y, w, h;
};

// Default member initializers, an array, a nested struct, bit-fields.
struct Config {
    int id = 7;
    const char* name = "none";
    int arr[3] = {0, 0, 0};
    Inner in{};
    unsigned flags : 4 = 0;
    unsigned mode : 3 = 1;
    double scale = 1.0;
};

// An aggregate with a base class.
struct Base {
    int p = 1;
};
struct Der : Base {
    int q = 2;
    int r = 3;
};

union Num {
    int i;
    float f;
};

// Anonymous members: their names are the fields of Anon.
struct Anon {
    int tag;
    union {
        int i;
        float f;
    };
    struct {
        int u, v;
    };
};

// Not an aggregate (a constructor): a keyed literal is written with assignments.
struct Ctor {
    int a;
    int b;
    Ctor() : a(5), b(6) {}
};

// An array field next to scalars.
struct Cfg {
    int id;
    int arr[3];
    Inner in;
};

inline int add(int a, int b) { return a + b; }
inline int mul(int a, int b) { return a * b; }
