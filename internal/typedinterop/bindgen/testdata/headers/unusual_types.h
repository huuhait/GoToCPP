#pragma once
#include <stddef.h>
#include <cstdint>
typedef float v4f __attribute__((vector_size(16)));
typedef float v4e __attribute__((ext_vector_type(4)));
typedef _BitInt(24) bi24;
struct W {
  v4f vec; v4e ev; _Atomic(int) at; __attribute__((aligned(16))) char al;
  wchar_t wc; char16_t c16; char32_t c32; char8_t c8; std::nullptr_t np; 
  int W::*pm; int (W::*pmf)(int); decltype(nullptr) dn; __fp16 half;
  long double ld; __int128 i128; unsigned __int128 u128;
  int (&ref)[3]; int &&rr; const volatile int cv;
  int (*fp_arr[2])(int);
  _Float16 f16;
};
int takes_vec(v4f);
int takes_ref(int &, const int &, int &&);
int (&retref(void))[3];
void noexcept_fn(int) noexcept;
[[noreturn]] void nr();
int __attribute__((visibility("default"))) vis(int);
extern "C" int c_linkage(int);
constexpr auto deduced = 42u;
inline constexpr float fconst = 1.5f;
template<class T> constexpr T tconst = T(3);
using Alias = int(*)(char);
using Arr3 = int[3];
void usesarr(Arr3 a, Arr3 *b);
