// Seeded mistake: a misspelled system header. The probe can't tell the
// names of the file then, so its namespace uses stay opaque. (Only the
// first header that is not found is reported: the compiler stops there.)
package main

import (
	"fmt"

	"<vectr>" // ERROR "cannot find header <vectr>"
)

func main() {
	v := std.vector[int]{1}
	fmt.Println(int(v.size()))
}
