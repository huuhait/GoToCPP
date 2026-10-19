#pragma once
extern struct { int x; int y; } glob_anon;
extern union { int i; float f; } glob_union[2];
typedef struct { int a; } *AnonPtr;
typedef struct { int a; } AnonArr[3];
typedef enum { ZA, ZB } ZE, *ZEP;
typedef union { int i; } UT;
struct Outer { struct { int in; } m; struct { int deep; } *mp; union { int a; } arr[2]; enum { Q1, Q2 } e[3]; };
inline auto lam() { struct L { int q; }; return L{1}; }
decltype(glob_anon) *get(void);
void takes(decltype(glob_anon) v);
static const auto cst = 5;
inline constexpr struct { int a; } konst = {3};
namespace ns { struct { int u; } nv; extern "C" int cfn(int); }
template<typename T> struct Box { T v; struct In { int q; }; };
Box<int> boxed;
Box<struct Outer> boxed2;
extern enum { GA, GB } genum[2];
typedef enum { TA, TB } TArr[3];
