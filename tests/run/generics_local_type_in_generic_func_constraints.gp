// phase: 07

// The constraint of a generic type declared inside a generic function can use the function's type parameters, and the
// instances made with the function's type parameter, a pointer to it, or a slice of it are distinct types.
package main

import "fmt"

type Num interface{ ~int | ~float64 }

func Sum[T Num](xs ...T) T {
	type Acc[A Num] struct {
		total A
		n     T
	}
	a := Acc[T]{}
	for _, x := range xs {
		a.total += x
		a.n++
	}
	fmt.Printf("%v %T\n", a, a)
	return a.total
}

func Same[T any]() any {
	type Box[U any] struct{ u U }
	return Box[T]{}
}

func Other[T any]() any {
	type Box[U any] struct{ u U }
	return Box[T]{}
}

func Wrappers[T any]() {
	type Box[U any] struct{ u U }
	fmt.Printf("%T %T %T\n", Box[T]{}, Box[*T]{}, Box[[]T]{})
}

func main() {
	fmt.Println(Sum(1, 2, 3), Sum(1.5, 2.5))
	fmt.Println(Same[int]() == Same[int](), Same[int]() == Same[string](), Same[int]() == Other[int]())
	Wrappers[int]()
	Wrappers[string]()
}

// Output:
// {6 3} main.Acc[int;int]
// {4 2} main.Acc[float64;float64]
// 6 4
// true false false
// main.Box[int;int] main.Box[int;*int] main.Box[int;[]int]
// main.Box[string;string] main.Box[string;*string] main.Box[string;[]string]
