// phase: 04

// A local header ("./vector.h") is found next to the .gp file, with -iquote and the package's
// folder: so the file "vector" in this folder (an #error) can't hide the system header
// <vector>, which both the program and vector.h include.
package main

import (
	"fmt"

	vec "./vector.h"
	"<vector>"
)

func main() {
	v := vec.vector_of(40, 2)
	var w std.vector[C.longlong] = v
	fmt.Println(int(vec.vector_sum(w)), int(std.size(w)))
}

// Output:
// 42 2
