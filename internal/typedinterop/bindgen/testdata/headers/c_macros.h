/* Macros: constants of every type, struct values, and the ones with no translation. */
#ifndef C_MACROS_H
#define C_MACROS_H

typedef struct Color { unsigned char r, g, b, a; } Color;
typedef struct Pair { int a, b; } Pair;
typedef struct Seg { Pair from, to; } Seg;
typedef union U { int i; float f; } U;
typedef struct Holder { int n; const char *name; Pair p; } Holder;
typedef struct Rowv { int v[4]; } Rowv;
typedef enum Mode { MODE_FAST = 1, MODE_SLOW = 2 } Mode;

#define VERSION "1.2.3"
#define VERSION_MAJOR 1
#define MASK (1u << 4 | 3)
#define NEG (-5)
#define BIGU 0xFFFFFFFFFFFFFFFFull
#define PI_F 3.14159f
#define PI_D 3.14159265358979
#define HALF 0.5
#define ONE_F 1.0
#define HUGE_D 1e300
#define CH 'A'
#define YES true
#define NO false
#define ESCAPES "line\n\ttab \"quoted\" \\ \x41\101"
#define LEN_OF (3 * sizeof(int))
#define DEFAULT_MODE MODE_SLOW
#define EMPTY
#define LINE_NO __LINE__
#define FILE_NAME __FILE__
#define FILE_ALIAS FILE_NAME
#define _KEPT_UNDERSCORE 7
#define __RESERVED 1

#define UNDEF_ME 1
#undef UNDEF_ME
#define REDEF 1
#undef REDEF
#define REDEF 2

#define MAX(a, b) ((a) > (b) ? (a) : (b))
#define NULL_PTR ((void *)0)
#define TYPE_ALIAS int
#define BRACE {
#define SEMI 1;
#define EXTERN_WORD extern

#define RED ((Color){ 255, 0, 0, 255 })
#define BLANK ((Color){ 0, 0, 0, 0 })
#define PARTIAL ((Color){ 255 })
#define ORIGIN ((Pair){ 0, 0 })
#define SEGMENT ((Seg){ { 1, 2 }, { 3, 4 } })
#define NEGPAIR ((Pair){ -1, -2 })
#define HOLDER ((Holder){ 1 })
#define UNION_VAL ((U){ .f = 1.5f })
#define ROW ((Rowv){ { 1, 2 } })
#define PAIRS ((Pair[2]){ { 1, 2 }, { 3, 4 } })

#endif
