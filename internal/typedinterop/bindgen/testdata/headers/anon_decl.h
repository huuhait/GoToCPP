#pragma once
struct Table {
  struct { int key; int val; } entries[16];
  struct { char c; } *cursor;
  struct { short a; } x, y;
  union { int i; float f; } vals[2][3];
  enum { E1, E2 } modes[4];
  int after;
};
struct NoUnion {
  struct { int key; int val; } entries[16];
  struct { char c; } *cursor;
  struct { short a; } x, y;
  struct { struct { char q; } inner[3]; short s; } nest[2];
  enum { N1, N2 } modes[4];
  int after;
};
namespace m1 { struct { int mm; } mv; }
namespace m2 { struct { char cc; } cv, *cp; }
typedef struct { int a; } NamedAnon;
extern struct { short b; } after_named;
extern NamedAnon plain_named;
