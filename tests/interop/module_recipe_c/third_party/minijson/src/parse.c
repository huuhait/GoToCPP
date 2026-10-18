#include <minijson/internal.h>

#include <math.h>
#include <stdlib.h>
#include <string.h>

static void push(mj_value *arr, mj_value *item, char *key) {
	arr->items = realloc(arr->items, sizeof(mj_value *) * (size_t)(arr->len + 1));
	if (arr->type == MJ_OBJECT) {
		arr->keys = realloc(arr->keys, sizeof(char *) * (size_t)(arr->len + 1));
		arr->keys[arr->len] = key;
	}
	arr->items[arr->len++] = item;
}

static mj_value *parse_string(const char **p) {
	const char *s = ++*p;
	while (**p && **p != '"')
		(*p)++;
	if (**p != '"')
		return NULL;
	mj_value *v = mj_new(MJ_STRING);
	v->string = mj_strndup(s, (int)(*p - s));
	(*p)++;
	return v;
}

mj_value *mj_parse_value(const char **p, int depth) {
	if (depth > MINIJSON_MAX_DEPTH)
		return NULL;
	mj_skip_space(p);
	switch (**p) {
	case '"':
		return parse_string(p);
	case '[':
	case '{': {
		int object = **p == '{';
		mj_value *arr = mj_new(object ? MJ_OBJECT : MJ_ARRAY);
		(*p)++;
		mj_skip_space(p);
		if (**p == (object ? '}' : ']')) {
			(*p)++;
			return arr;
		}
		for (;;) {
			char *key = NULL;
			mj_skip_space(p);
			if (object) {
				mj_value *k = **p == '"' ? parse_string(p) : NULL;
				if (!k) {
					mj_free(arr);
					return NULL;
				}
				key = k->string;
				k->string = NULL;
				mj_free(k);
				mj_skip_space(p);
				if (*(*p)++ != ':') {
					free(key);
					mj_free(arr);
					return NULL;
				}
			}
			mj_value *item = mj_parse_value(p, depth + 1);
			if (!item) {
				free(key);
				mj_free(arr);
				return NULL;
			}
			push(arr, item, key);
			mj_skip_space(p);
			if (**p == ',') {
				(*p)++;
				continue;
			}
			if (**p == (object ? '}' : ']')) {
				(*p)++;
				return arr;
			}
			mj_free(arr);
			return NULL;
		}
	}
	default: {
		char *end;
		double d = strtod(*p, &end);
		if (end == *p)
			return NULL;
		*p = end;
		mj_value *v = mj_new(MJ_NUMBER);
		v->number = d;
		return v;
	}
	}
}

mj_value *mj_parse(const char *text) {
	const char *p = text;
	mj_value *v = mj_parse_value(&p, 1);
	if (v) {
		mj_skip_space(&p);
		if (*p) {
			mj_free(v);
			return NULL;
		}
	}
	return v;
}
