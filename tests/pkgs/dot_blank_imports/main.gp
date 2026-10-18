// phase: 06

// Dot imports (the names of the package are used unqualified, a function, a variable, a type, a constant) and blank imports (the package is initialized for its side effects) of packages of the module.

package main

import (
	"fmt"

	. "example.com/imports/dot"
	_ "example.com/imports/side"
)

func main() {
	p := Make(1, K)
	fmt.Println(p.Sum(), Counter, Variadic(1, 2, 3))
	Counter++
	var q Pair = Pair{A: 2}
	fmt.Println(q, Counter)
	f := Make
	fmt.Println(f(3, 4).Sum())
}

// Output:
// init dot
// side effect
// init side
// 8 100 3
// {2 0} 101
// 7
