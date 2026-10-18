// phase: 07

// Deferred calls of another package's generic functions that call recover: explicit and inferred type arguments, and a func value.
package main

import (
	"fmt"

	"example.com/grec/lib"
)

type MyErr struct{ code int }

func f1() (out string) {
	defer lib.Catch[MyErr](&out)
	panic(MyErr{7})
}

func f2() (out string) {
	defer func() { out += fmt.Sprint(" outer ", recover()) }()
	defer lib.Catch[string](&out)
	panic(MyErr{8})
}

func f3() {
	defer lib.Swallow("inferred")
	panic("a")
}

func f4() {
	s := lib.Swallow[int]
	defer s(4)
	panic("b")
}

func main() {
	fmt.Println(f1())
	fmt.Println(f2())
	f3()
	f4()
}

// Output:
// caught {7}
//  outer {8}
// swallowed inferred a
// swallowed 4 b
