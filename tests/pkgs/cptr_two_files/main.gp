// phase: 06

// Pointers to C structs that gocpp owns (step 4.10, the registry of the objects it made with &T{...}):
// list/a.gp makes the nodes and stores them into members of each other, list/b.gp (another translation
// unit, another file of the package) reads the members back as pointers. The registry is one for the
// program, and the header of package list includes umem.hpp for the packages that use its Handle.
package main

import (
	"fmt"

	"example.com/cptr/list"
)

func main() {
	h := list.New()
	for i := 1; i <= 5; i++ {
		h.Push(i * i)
	}
	fmt.Println(h.Sum())
	h.Drop()
	h2 := list.New()
	h2.Push(7)
	fmt.Println(h.Sum(), h2.Sum())
}

// Output:
// 55
// 30 7
