// phase: 01

// A name used like a namespace that the headers don't declare as one is
// undefined (the namespace probe, Phase 01 README §2.5 step 4), at each
// use, like any undefined name: typos of namespaces and of packages.
package main

import (
	"fmt"

	"<vector>"
)

func main() {
	v := std.vector[int]{1}
	w := stdd.vector[int]{2} // ERROR "undefined: stdd"
	var x stdd.size_t        // ERROR "undefined: stdd"
	fmt.Println(v, w, x)
	fmtt.Println(v) // ERROR "undefined: fmtt"
}
