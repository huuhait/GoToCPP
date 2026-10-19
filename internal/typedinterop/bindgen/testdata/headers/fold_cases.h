/* struct macros whose values the generator computes: C's conversions and operators */
#include <stdint.h>
#include <stddef.h>
typedef struct { size_t a; uint32_t b; uint64_t c; double d; long e; char f; } Lim;
#define LIM_ALL (Lim){ (size_t)-1, (uint32_t)-1, (uint64_t)~0, 0.1f, (long)2.5, '\xff' }
typedef struct { uint64_t m; uint32_t n; } U;
#define U_NEG (U){ -1, -2 }
typedef struct { int8_t a; uint8_t b; int16_t c; int32_t d; } Small;
#define SMALL_CAST (Small){ (int8_t)200, (uint8_t)(uint32_t)0x1234, (int16_t)-70000, -((uint32_t)4294967295) }
#define SMALL_NOT (Small){ ~(int8_t)200, 0, 0, 0 }
typedef struct { long a, b, c, d; short e; } Div;
#define DIV_OK (Div){ -7/2, -7%3, (long)(3.14159), (long)(1.0/3), (short)(-2.5) }
typedef struct { int a, b; } Two;
#define BANG (Two){ !5, !0 }
#define DIV_ZERO (Two){ 1/0, 2 }
#define REM_ZERO (Two){ 1%0, 2 }
#define SHIFT_LIMIT (Two){ 1<<70, 2 }
#define SHIFT_NEG (Two){ 1<<-1, 2 }
#define INT_OVF (Two){ 2147483647+1, 2 }
#define MIN_DIV (Two){ (-2147483647-1)/-1, 2 }
#define FLOAT_BIG (Two){ (int)1e30, 2 }
typedef struct { float f; double d; } FD;
#define F32 (FD){ (float)1/3, 0.1f }
#define F32_2 (FD){ 0.1f, 1.0/3 }
#define F_NEG (FD){ -0.5f, -2 }
#define INT2FLOAT (FD){ 16777217, 9007199254740993LL }
/* fields whose constants the view cannot write, and fields that cut the value */
typedef struct { __int128 x; } I128;
#define I1 (I128){ 5 }
#define I2 (I128){ (__int128)(0.1) }
typedef struct { long double ld; bool b; } LDB;
#define LD1 (LDB){ 0.1, true }
enum ES { E_A, E_B = 5, E_C = -3 };
typedef struct { long double a; } LDE;
#define LDE1 (LDE){ E_C }
enum EC2 : unsigned char { EC2_A = 1 };
typedef struct { EC2 e; } HasEC;
#define EC_WRAP (HasEC){ (EC2)(+(2147483648)) }
#define EC_OK (HasEC){ (EC2)200 }
typedef struct { unsigned long long c : 40; int z; } BF;
#define BF1 (BF){ 0x1ffffffffffULL, 3 }
typedef struct { signed int s : 4; } SB;
#define SB1 (SB){ 9 }
typedef struct { unsigned long a; int b; double c; } SZ;
struct Big { char x[40]; };
#define SZ1 (SZ){ sizeof(struct Big), sizeof(int) * 2, alignof(double) }
#define NEG_SINGLE (FD){ -0.3f, -0.3 }
typedef uint32_t U32;
typedef struct { U32 a; } UT;
#define NOOP (UT){ (U32)7 }
#define NAN_INT (Two){ (int)(0.0 / 0.0), 1 }
#define INF_INT (Two){ (int)(1.0 / 0.0), 1 }
#define BIG_LIT (Two){ (int)1e999, 1 }
typedef struct { long double a[2]; } LDA;
#define LDA1 (LDA){ { 1.5, 2 } }
