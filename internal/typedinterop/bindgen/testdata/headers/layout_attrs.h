#pragma once
#include <stdint.h>
#include <stdarg.h>
#include <stddef.h>
#ifdef __cplusplus
extern "C" {
#endif
struct __attribute__((packed)) Packed { char c; int i; short s; };
struct Aligned { char c; } __attribute__((aligned(16)));
struct alignas(32) Aligned32 { int x; };
struct Bits { unsigned a:3; unsigned b:5; int c:7; unsigned long long d:40; char e; };
struct Flex { int n; char data[]; };
struct Zero { int n; int z[0]; };
union U { int i; float f; char c[8]; };
struct Anon { int tag; union { int i; float f; }; struct { short a, b; }; struct { char x; } named; union { double d; long l; } un; };
typedef struct { int a; } Tdef;
typedef struct Fwd Fwd;
struct Fwd { Fwd *next; Tdef t; void (*cb)(int, ...); };
typedef int (*Cb)(void *ud, const char *fmt, va_list ap);
typedef void (Fn)(int);
typedef Fn *FnPtr;
typedef void (*Sig)(int);
Sig mysignal(int, Sig);
int vprintfish(const char *fmt, va_list ap);
int printfish(const char *fmt, ...);
void takes_arr(int a[4], int b[], const int (*m)[3]);
char (*retarr(void))[4];
struct Nested { struct Inner { int q; } in; enum Color { RED, GREEN } col; struct Inner *p; };
typedef struct Node Node;
struct Node { Node *l, *r; int v; };
extern int arr_global[10];
extern const char *const names[];
struct HasEnum { enum { A1 = 1, A2 } kind; unsigned char flags; bool ok; long double ld; __int128 big; };
struct Align2 { char c; alignas(8) char d; };
typedef long long ll_t;
typedef unsigned long ulong_t;
typedef struct { unsigned char r, g, b, a; } Color;
struct Arr2 { Color pal[4][2]; int8_t i8; uint16_t u16; int64_t i64; size_t sz; intptr_t ip; };
struct Pad { char a; double b; };
#ifdef __cplusplus
}
#endif
