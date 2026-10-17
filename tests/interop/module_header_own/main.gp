// phase: 04

// Module headers (Phase 06 step 6.1b, README §2.11): a header of the project's own module is imported by the
// module path, like a package of it. native/counter.hpp includes native/step.hpp by its name in the module,
// which works because the module's folder is on the include path. native/cadd.h is a C header without a
// __cplusplus guard: it is included inside extern "C", as a local header would be. A "./local.h" import
// next to it stays a header of the package folder, and the native/counter.hpp of the package folder is
// not the module's (there is none here: the module and the package share the folder).
package main

import (
	"fmt"

	"./local.h"
	"example.com/headers/native/cadd.h"
	"example.com/headers/native/counter.hpp"
)

func main() {
	fmt.Println(counter.counter_next(1), counter.counter_next(counter.counter_next(1)))
	fmt.Println(cadd.c_add(2, 3))
	fmt.Println(local.local_value())
}

// Output:
// 3 5
// 5
// 42
