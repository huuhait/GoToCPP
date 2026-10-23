// phase: 07

// The mistakes with a generic type declared inside a function are Go's own (go/types words them): no instantiation, too many type
// arguments, a type argument that does not satisfy the constraint, a type that mentions the function's type parameter.
package main

import "fmt"

func f() {
	type Box[T any] struct{ v T }
	var a Box // ERROR "cannot use generic type Box\[T any\] without instantiation"
	var b Box[int, string] // ERROR "too many type arguments for type Box: have 2, want 1"
	var c Box[[]int] = Box[int]{1} // ERROR "cannot use Box\[int\]\{…\} \(value of struct type Box\[int\]\) as Box\[\[\]int\] value in variable declaration"
	type Num[T int | float64] struct{ n T }
	var d Num[string] // ERROR "string does not satisfy int \| float64"
	fmt.Println(a, b, c, d)
}

func g[T any]() {
	type Pair[U any] struct {
		t T
		u U
	}
	var p Pair[int]
	p.t = 1 // ERROR "cannot use 1 \(untyped int constant\) as T value in assignment"
	_ = p
}

func main() {
	f()
	g[int]()
}
