package b

import "github.com/example/minijson/include/minijson/minijson.h"

func Sum(s string) float64 {
	v := minijson.mj_parse(C.cstr(s))
	if v == nil {
		return -1
	}
	defer minijson.mj_free(v)
	return float64(minijson.mj_sum(v))
}
