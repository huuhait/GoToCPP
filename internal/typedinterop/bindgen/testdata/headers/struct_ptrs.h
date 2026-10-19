#include <stddef.h>
typedef struct { const char *s; void *p; int (*cb)(int); } P;
typedef struct { char name[8]; int n; } S8;
typedef struct { const char *names[2]; } SA;
#define P_NULL (P){ "s", NULL, NULL }
#define P_ZERO (P){ 0, 0, 0 }
#define P_NULLPTR (P){ nullptr, nullptr, nullptr }
#define S8_M (S8){ "abc", 1 }
#define S8_M2 (S8){ {'a','b'}, 1 }
#define SA_M (SA){ { "x", "y" } }
typedef struct { const char *names[2]; void (*cbs[2])(int); } SA2;
#define SA2_ZERO (SA2){ { 0, 0 }, { 0, 0 } }
