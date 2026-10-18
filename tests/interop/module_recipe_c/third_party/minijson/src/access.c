#include <minijson/internal.h>

#include <math.h>
#include <string.h>

mj_type mj_typeof(const mj_value *v) { return v ? v->type : MJ_NULL; }
double mj_number(const mj_value *v) { return v && v->type == MJ_NUMBER ? v->number : NAN; }
int mj_len(const mj_value *v) { return v ? v->len : 0; }
const char *mj_string(const mj_value *v) { return v && v->type == MJ_STRING ? v->string : NULL; }

const mj_value *mj_at(const mj_value *v, int i) {
	return v && i >= 0 && i < v->len ? v->items[i] : NULL;
}

const mj_value *mj_get(const mj_value *v, const char *key) {
	if (!v || v->type != MJ_OBJECT)
		return NULL;
	for (int i = 0; i < v->len; i++)
		if (strcmp(v->keys[i], key) == 0)
			return v->items[i];
	return NULL;
}
