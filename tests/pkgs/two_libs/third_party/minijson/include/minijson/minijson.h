#ifndef MINIJSON_H
#define MINIJSON_H

/* A tiny JSON reader (a test library in the layout of a real one). */

#ifdef __cplusplus
extern "C" {
#endif

#ifndef MINIJSON_MAX_DEPTH
#error "MINIJSON_MAX_DEPTH must be defined: the recipe's define reaches both the library and the user"
#endif

typedef enum { MJ_NULL, MJ_NUMBER, MJ_STRING, MJ_ARRAY, MJ_OBJECT } mj_type;

typedef struct mj_value mj_value;

mj_value *mj_parse(const char *text); /* NULL on a syntax error */
void mj_free(mj_value *v);
mj_type mj_typeof(const mj_value *v);
double mj_number(const mj_value *v);
int mj_len(const mj_value *v);
const mj_value *mj_at(const mj_value *v, int i);
const mj_value *mj_get(const mj_value *v, const char *key);
const char *mj_string(const mj_value *v);
double mj_sum(const mj_value *array);
int mj_nesting(const mj_value *v);

static inline int mj_max_depth(void) { return MINIJSON_MAX_DEPTH; }

#ifdef __cplusplus
}
#endif

#endif
