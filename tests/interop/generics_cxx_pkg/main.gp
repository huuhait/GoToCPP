// phase: 07

// Generic functions of another package, called with C++ values: main's unit has the function templates.
package main

import (
	"fmt"

	"example.com/gcxx/util"

	"<string>"
	"<vector>"
)

func main() {
	v := std.vector[int]{1, 2, 3}
	s := std.string("hello")
	fmt.Println(int(util.Id(v).size()), int(util.Id[std.string](s).size()))
	n, str := util.Pair(s, 4)
	fmt.Println(n, int(str.size()))
	var ls []int = util.Map([]std.string{s, std.string("ab")}, func(x std.string) int { return int(x.size()) })
	fmt.Println(ls)
	var ds []std.vector[int] = util.Dup(v)
	fmt.Println(len(ds), int(ds[1].size()))
}

// Output:
// 3 5
// 4 5
// [5 2]
// 2 3
