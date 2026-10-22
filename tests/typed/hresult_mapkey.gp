// phase: 12
// opaque: same
// windows: yes

// The key of a map literal can be a C/C++ value (the NTSTATUS here is a plain number), and the
// values next to it can be elided literals: of a named map type, and two levels deep. The literals
// have the types the map gives them (check types them, though go/types skips them next to such a key).
package main

import (
	"fmt"

	"./hresult.hpp"
)

type Point struct{ X, Y int }

type Points map[any]Point

func main() {
	k := hresult.nt_result(1)
	named := Points{k: {1, 2}}
	nested := map[any]map[any][]int{k: {k: {7, 8}}}
	fmt.Println(len(named), len(nested))
	fmt.Println(named[k].Y, nested[k][k][1])
}

// Output:
// 1 1
// 2 8
