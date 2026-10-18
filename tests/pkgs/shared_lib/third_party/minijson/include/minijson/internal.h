#ifndef MINIJSON_INTERNAL_H
#define MINIJSON_INTERNAL_H

#include <minijson/minijson.h>

#ifndef MINIJSON_BUILDING
#error "internal.h is for the library's own files (private-define MINIJSON_BUILDING)"
#endif

struct mj_value {
	mj_type type;
	double number;
	char *string;
	int len;
	struct mj_value **items; /* array items and object values */
	char **keys;             /* object keys */
};

mj_value *mj_new(mj_type type);
char *mj_strndup(const char *s, int n);
mj_value *mj_parse_value(const char **p, int depth);
void mj_skip_space(const char **p);

#endif
