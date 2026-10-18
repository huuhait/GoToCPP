// phase: 04

// A C library built from a cxx recipe (Phase 06 step 6.1d): third_party/minijson is a small JSON library
// in the layout of a real one (src/*.c, include/minijson/*.h), with no gocpp.mod of its own. The
// recipe in this project's gocpp.mod says how to build it: the C files, the include folder, a macro for
// both sides (MINIJSON_MAX_DEPTH is in the header and in the library), one for the library alone, and
// -lm. gocpp compiles each C file once, into the object cache, and links the objects.
package main

import (
	"fmt"

	"github.com/example/minijson/include/minijson/minijson.h"
)

func main() {
	v := minijson.mj_parse("{\"name\": \"gocpp\", \"nums\": [1, 2, 3.5], \"nested\": {\"x\": 4}}")
	nums := minijson.mj_get(v, "nums")
	nested := minijson.mj_get(v, "nested")
	fmt.Println(minijson.mj_len(nums), minijson.mj_number(minijson.mj_at(nums, 2)), minijson.mj_number(minijson.mj_get(nested, "x")))
	fmt.Println(minijson.mj_sum(nums), minijson.mj_max_depth(), minijson.mj_nesting(v))
	minijson.mj_free(v)

	bad := minijson.mj_parse("[1, 2,")
	fmt.Println(bad == nil, minijson.mj_nesting(nil))
}

// Output:
// 3 3.5 4
// 6.5 16 2
// true 0
