#include <minijson/internal.h>

#include <stdlib.h>
#include <string.h>

mj_value *mj_new(mj_type type) {
	mj_value *v = calloc(1, sizeof *v);
	v->type = type;
	return v;
}

char *mj_strndup(const char *s, int n) {
	char *d = malloc((size_t)n + 1);
	memcpy(d, s, (size_t)n);
	d[n] = 0;
	return d;
}

void mj_skip_space(const char **p) {
	while (**p == ' ' || **p == '\n' || **p == '\t' || **p == '\r')
		(*p)++;
}

void mj_free(mj_value *v) {
	if (!v)
		return;
	for (int i = 0; i < v->len; i++) {
		mj_free(v->items[i]);
		if (v->keys)
			free(v->keys[i]);
	}
	free(v->items);
	free(v->keys);
	free(v->string);
	free(v);
}
