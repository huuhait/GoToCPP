// phase: 04

// An iterator plus a gocpp int is "mismatched types" (C++ would convert the int without a word);
// the gocpp side converts to a C type: v.begin() + C.long(k).
package main

import (
	"fmt"

	"<algorithm>"
	"<vector>"
)

func main() {
	v := std.vector[int](5, 0)
	for i := 0; i < 5; i++ {
		v[i] = 10 - i
	}
	k := 3
	std.sort(v.begin(), v.begin()+C.long(k))
	fmt.Println(int(v[0]), int(v[1]), int(v[2]), int(v[3]), int(v[4]))
	fmt.Println(int(std.distance(v.begin(), v.end()-C.long(1))))
}

// Output:
// 8 9 10 7 6
// 4
