#pragma once
typedef struct { unsigned char r, g, b, a; } Color;
typedef struct { float x, y; } V2;
typedef struct { V2 pos; V2 size; } Rect;
typedef struct { Rect r; Color c; int n; } Deep;
typedef struct { int a, b, c, d; } Four;
typedef struct { int tag; int arr[3]; } WithArr;
typedef struct { char name[8]; double d; bool ok; } WithStr;
typedef struct { const char *s; void *p; int (*cb)(int); } WithPtr;
enum Kind { K_A, K_B = 7 };
typedef struct { enum Kind k; unsigned flags : 4; unsigned more : 4; int z; } WithBits;
union UU { int i; float f; unsigned char b[4]; };
typedef struct { union { int i; float f; }; int after; } WithAnon;
typedef struct { Color pal[2]; V2 pts[2][2]; } Arrs;
typedef struct { long long big; unsigned long long ubig; short s; signed char sc; } Nums;
typedef struct { double a, b; } Pair;
#define SM_COLOR (Color){ 1, 2, 3, 4 }
#define SM_V2 ((V2){ -1.5f, 2.25f })
#define SM_RECT (Rect){ {1, 2}, {3, 4} }
#define SM_RECT2 (Rect){ .size = {3, 4} }
#define SM_DEEP (Deep){ { {1, 2}, {3, 4} }, { 9, 8, 7, 6 }, 42 }
#define SM_PARTIAL (Four){ 1, 2 }
#define SM_PARTIAL_D (Four){ .c = 3 }
#define SM_EMPTY (Four){ 0 }
#define SM_EMPTY2 (Four){}
#define SM_ARR (WithArr){ 5, {1, 2} }
#define SM_ARR_FULL (WithArr){ 5, {1, 2, 3} }
#define SM_STR (WithStr){ "hi", 1.5, true }
#define SM_PTR (WithPtr){ "s", 0, 0 }
#define SM_BITS (WithBits){ K_B, 3, 4, 9 }
#define SM_UNION (UU){ .f = 1.5f }
#define SM_UNION_I (UU){ 7 }
#define SM_ANON (WithAnon){ { 3 }, 4 }
#define SM_ARRS (Arrs){ { {1,2,3,4}, {5,6,7,8} }, { {{0,1},{2,3}}, {{4,5},{6,7}} } }
#define SM_NUMS (Nums){ -9223372036854775807LL-1, 18446744073709551615ULL, -3, -4 }
#define SM_PAIR (Pair){ 1.0/3.0, -0.0 }
#define SM_NEST_CALL (Four){ (1+2), (3*4), -5, ~0 }
#define SM_PAIR_EXPR ((Pair){ 2*3.5, 1e300 })
#define SM_CONSTEXPR_REF Four{ K_B, 2, 3, 4 }
extern const Color gcolor;
typedef struct { int a, b; } TS;
#define TILDE_S (TS){ ~0, -1 }
#define NEG_ZERO_PAIR ((Pair){ 1.0, -0.0 })
typedef struct { unsigned a, b; } TU;
#define TILDE_U (TU){ ~0u, ~0 }
#define SHIFT_S (TS){ 1 << 4, 256 >> 4 }
