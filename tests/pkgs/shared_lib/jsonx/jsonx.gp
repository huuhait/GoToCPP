package jsonx

import "github.com/example/minijson/include/minijson/minijson.h"

// Sum parses a JSON array of numbers and adds them up.
func Sum(s string) float64 {
	v := minijson.mj_parse(C.cstr(s))
	if v == nil {
		return -1
	}
	defer minijson.mj_free(v)
	return float64(minijson.mj_sum(v))
}

func MaxDepth() int { return int(minijson.mj_max_depth()) }
