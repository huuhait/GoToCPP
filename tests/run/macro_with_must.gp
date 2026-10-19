// phase: 05

// The built-in macros together with the must operator `!` (Phase 10 step 10.8): both are rewrites, and a
// macro in the arguments of a must call or inside a must function expands in place.

package main

import (
	"errors"
	"fmt"
)

func parse!(s string) (int, error) {
	if s == "" {
		return 0, errors.New(fmt.Sprint("empty at line ", #line(), " in ", #func()))
	}
	return len(s), nil
}

func use!(s string) (int, error) {
	n := parse!(s)
	fmt.Println(#text(parse!(s)), n, #func())
	return n + #line(), nil
}

func main() {
	n, err := use("abc")
	fmt.Println(n, err)
	_, err = use("")
	fmt.Println(err)
}

// Output:
// parse!(s) 3 main.use
// 26 <nil>
// empty at line 15 in main.parse
