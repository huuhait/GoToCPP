// phase: 07

// An instance that holds an unnamed struct by value (alone, over a type parameter, or holding another instance), reached only through a slice of another generic type: the unnamed struct is complete before the instance, whatever order the header finds them in. main names only the outer types (naming an inner instance there would hide the ordering bug).
package main

import "fmt"

type A[T any] struct {
	s struct{ n int }
	v T
}
type HoldA[T any] struct{ as []A[T] }

type B[T any] struct {
	s struct{ n T }
}
type HoldB[T any] struct{ bs []B[T] }

type Cell[T any] struct{ c T }
type D[T any] struct{ s struct{ c Cell[T] } }
type HoldC[T any] struct{ cs []D[T] }

func main() {
	var a HoldA[int]
	var b HoldB[int]
	var c HoldC[string]
	fmt.Println(a, b, c)
	fmt.Printf("%T %T %T\n", a, b, c)
}

// Output:
// {[]} {[]} {[]}
// main.HoldA[int] main.HoldB[int] main.HoldC[string]
