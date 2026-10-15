// phase: 04
// ExitCode: 2
// Stderr: (?m)^panic: runtime error: index out of range \[5\] with length 2$

// A gocpp slice, array or map of C/C++ values is a gocpp value: range copies its elements,
// slicing shares the array, v, ok := m[k] is Go's comma-ok, and indexes are bounds-checked.
// (A range over a C++ container is step 4.7.)
package main

import (
	"fmt"

	"<string>"
)

func main() {
	var arr [3]C.int
	arr[1] = 5
	for i, v := range arr {
		fmt.Println(i, int(v))
	}
	var ss []std.string
	ss = append(ss, std.string("a"), std.string("bc"))
	n := 0
	for _, s := range ss {
		n += int(s.size())
		s += "zz" // a copy: ss doesn't change
	}
	fmt.Println(n, int(ss[0].size()))
	tail := ss[1:]
	tail[0] += "!" // shares ss's array
	fmt.Println(len(tail), int(ss[1].size()))
	part := arr[:2]
	part[0] = 9
	fmt.Println(int(arr[0]), len(part))
	cm := make(map[string]C.int)
	cm["a"] = 4
	v, ok := cm["a"]
	w, ok2 := cm["b"]
	fmt.Println(int(v), ok, int(w), ok2)
	for k, x := range cm {
		fmt.Println(k, int(x))
	}
	fmt.Println(int(part[5]))
}

// Output:
// 0 0
// 1 5
// 2 0
// 3 1
// 1 3
// 9 2
// 4 true 0 false
// a 4
