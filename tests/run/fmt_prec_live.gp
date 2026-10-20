// phase: 09

package main

import "fmt"

var a []any

type S struct{}

func (S) String() string { a[1] = 1; return "S" }

type T struct{}

func (T) String() string { a[1] = "x"; return "T" }

func main() {
	a = []any{S{}, 4, 3.14159265}
	fmt.Printf("%v %.*f\n", a...)
	a = []any{T{}, 4, 3.14159265}
	fmt.Printf("%v %.*f\n", a...)
}

// Output:
// S 3.1
// T %!(BADPREC)3.141593
