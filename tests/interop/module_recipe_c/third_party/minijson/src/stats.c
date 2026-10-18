#include <minijson/internal.h>

#include <math.h>

double mj_sum(const mj_value *array) {
	double t = 0;
	for (int i = 0; i < mj_len(array); i++)
		t += mj_number(mj_at(array, i));
	return t;
}

int mj_nesting(const mj_value *v) {
	if (!v || (v->type != MJ_ARRAY && v->type != MJ_OBJECT))
		return 0;
	int best = 0;
	for (int i = 0; i < v->len; i++) {
		int d = mj_nesting(v->items[i]);
		if (d > best)
			best = d;
	}
	return best + 1;
}

#ifdef MINIJSON_UNIX
int mj_unix_marker(void) { return 1; }
#endif
