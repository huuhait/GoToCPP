package stats

import "github.com/example/minijson/include/minijson/minijson.h"

// Len is the length of the array in s, with the same library as package jsonx (one build of it).
func Len(s string) int {
	v := minijson.mj_parse(C.cstr(s))
	if v == nil {
		return -1
	}
	defer minijson.mj_free(v)
	return int(minijson.mj_len(v))
}
