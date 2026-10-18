/* A plain C header for c_unions.gp (step 4.18): unions, bit-fields, anonymous members, enums
   with values, function-like and constant macros, nested structs, arrays and function pointers
   in structs. It parses as C, so gocpp wraps it in extern "C". */
#ifndef CUNION_H
#define CUNION_H

#include <stdint.h>
#include <string.h>

typedef union {
    int32_t i;
    float f;
    unsigned char bytes[4];
} Num;

typedef struct {
    unsigned int ready : 1;
    unsigned int mode : 3;
    unsigned int level : 4;
    int signed_part : 5;
} Flags;

typedef struct {
    int kind;
    union {
        int ival;
        double dval;
    }; /* anonymous union member */
    struct {
        int x, y;
    } pos; /* a nested struct */
    char name[8];
    int (*op)(int, int);
} Item;

enum Weekday { MON = 1, TUE, WED, SUN = 7 };
typedef enum { OFF, ON = 10 } Switch;

#define CU_VERSION 3
#define CU_NAME "cunion"
#define CU_PI 3.5
#define CU_FLAG(n) (1u << (n))
#define CU_SQUARE(x) ((x) * (x))
#define CU_MAX(a, b) ((a) > (b) ? (a) : (b))

static inline int cu_add(int a, int b) { return a + b; }
static inline int cu_mul(int a, int b) { return a * b; }

static inline Num cu_float_bits(float f) {
    Num n;
    n.f = f;
    return n;
}

static inline Item cu_item(int kind, double d) {
    Item it;
    memset(&it, 0, sizeof it);
    it.kind = kind;
    it.dval = d;
    it.pos.x = kind * 10;
    it.pos.y = kind * 100;
    strcpy(it.name, "item");
    it.op = kind % 2 ? cu_add : cu_mul;
    return it;
}

static inline int cu_apply(const Item* it, int a, int b) { return it->op(a, b); }
static inline Flags cu_flags(unsigned mode) {
    Flags f = {0};
    f.ready = 1;
    f.mode = mode;
    return f;
}


/* Text members of other types, for c_string_compare_error.gp. */
typedef struct {
    char tag[4];
    char tag2[6];
} Tag;
static inline Tag cu_tag(void) {
    Tag t = {"abc", "tag2"};
    return t;
}
static inline const char* cu_label(void) { return "label"; }

#endif
