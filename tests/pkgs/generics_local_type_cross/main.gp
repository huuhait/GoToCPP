// phase: 07

// Generic types declared in functions of another package: the instance has one identity wherever it is made
// (this package asks for the generic function, and the package that declares the type names it as Go does), and
// a type of the same name declared here is another type.
package main

import (
	"fmt"

	"example.com/gltcross/lib"
)

func Make[T any](v T) any {
	type Box[U any] struct {
		u U
		t T
	}
	return Box[string]{"main", v}
}

func main() {
	a, b, c := lib.Make(1), lib.Make("x"), lib.Make(1)
	fmt.Println(lib.Describe(a), "|", lib.Describe(b))
	fmt.Println(a == c, a == b, a == Make(1))
	fmt.Println(lib.Describe(Make(1)))
	fmt.Println(lib.Describe(lib.Plain()), lib.Plain() == lib.Plain())
	fmt.Println(lib.Describe(lib.NewStack(1.5).Cell()), lib.Describe(lib.NewStack("s").Cell()))
	fmt.Println(lib.NewStack(1).Cell() == lib.NewStack(1).Cell(), lib.NewStack(1).Cell() == lib.NewStack(2).Cell())
}

// Output:
// lib.Box[int;string] {lib 1} | lib.Box[string;string] {lib x}
// true false false
// main.Box[int;string] {main 1}
// lib.Box[int] {1} true
// lib.Cell[float64;bool] {true 1.5} lib.Cell[string;bool] {true s}
// true false
