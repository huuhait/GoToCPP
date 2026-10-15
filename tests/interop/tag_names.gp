// phase: 04

// cgo's tag names are qualified in C++ (struct ::point), so a gocpp type of the same name
// (point) in the package's namespace can't hide them.
package main

import (
	"fmt"

	"./tags.h"
)

type point struct{ X, Y int }

func main() {
	var p tags.struct_point = tags.make_point(3, 4)
	var m tags.enum_mode = tags.MODE_B
	var q point = point{1, 2}
	fmt.Println(int(tags.point_sum(p)), int(m), q.X+q.Y)
}

// Output:
// 7 1 3
