/* A C header with the usual shapes of C declarations. */
#ifndef C_BASIC_H
#define C_BASIC_H

#include <stdarg.h>
#include <stddef.h>
#include <stdint.h>
#include <stdio.h>

/* a struct with its typedef (same name), an unnamed one, a forward declaration */
typedef struct Vec2 { float x, y; } Vec2;
typedef struct { unsigned char r, g, b, a; } Rgba;
typedef struct Opaque Opaque;
typedef struct Node Node;
struct Node { Node *next; int values[4]; Vec2 pos; };

/* a tag with a typedef of another name; a struct that is named like a function */
struct tagged { int a; };
typedef struct tagged tagged_t;
struct stat { int mode; long size; };
int stat(const char *path, struct stat *buf);

/* enums: plain, typedef'd with a negative value, anonymous, one that needs 64 bits */
enum Color { RED, GREEN = 5, BLUE };
typedef enum { MODE_A = 1, MODE_B = 2, MODE_NEG = -1 } Mode;
enum { ANON_ONE = 1, ANON_TWO = 2 };
typedef enum Big { BIG_LOW = 0, BIG_HIGH = 0x1FFFFFFFFLL } Big;

union Num { int i; float f; };
struct Packed { unsigned a : 3; unsigned b : 5; int c; };
struct WithAnon { int k; union { int i; float f; }; struct { short x, y; } pos; };
struct map { int go; int select; };

typedef int (*Callback)(int code, const char *msg, void *user);
typedef void (*Handler)(int);
typedef const char *CString;
typedef unsigned long ulong_t;
typedef Vec2 Point;

/* names that start with an underscore are private: written only when something else uses them */
struct _unused_rec { int a; };
static int _hidden_fn(void);
struct _private_rec { int v; };
typedef struct _private_rec Public;
int init(int mode);

int add(int a, int b);
double scale(Vec2 v, double k);
Vec2 make_vec(float x, float y);
void log_message(const char *fmt, ...);
int vlog(const char *fmt, va_list ap);
size_t count_bytes(const uint8_t *data, size_t n, uint32_t *out);
void *alloc(size_t n);
int register_cb(Callback cb, void *user);
void set_handler(Handler h);
void take_array(int values[], int n);
void take_matrix(int m[3][3]);
FILE *open_file(const char *path);
const char *const *names(void);
int unnamed_params(int, const char *, Vec2);
int range(int type, int func);

extern int global_counter;
extern const char *global_name;
extern Vec2 origin;
extern int table[16];
extern int unknown_size[];

static inline int twice(int x) { return 2 * x; }

#endif
