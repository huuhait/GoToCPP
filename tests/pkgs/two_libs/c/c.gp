package c

import (
	"github.com/example/minijson/include/minijson/minijson.h"
	"github.com/example/tinyc/include/tinyc/tinyc.h"
)

// Both libraries; minijson is also used by package b: each library is built once.
func Both(s string) float64 {
	v := minijson.mj_parse(C.cstr(s))
	if v == nil {
		return -1
	}
	defer minijson.mj_free(v)
	return float64(minijson.mj_sum(v)) + float64(tinyc.tinyc_scale(14))
}
