// phase: 07

// A generic function is no value until it is instantiated, and generic code can't be assigned or called with the wrong shape.
package main

import "fmt"

func Id[T any](x T) T { return x }

func Apply[T any](f func(T) T, x T) T { return f(x) }

func Map[T, U any](xs []T, f func(T) U) []U { return nil }

func main() {
	f := Id // ERROR "cannot use generic function Id without instantiation"
	var g func(int) int = Id
	var h func(int) string = Id // ERROR "inferred type func\(x int\) int for func\(x T\) T does not match type func\(int\) string of h"
	fmt.Println(f, g(1), h)
	fmt.Println(Apply(Id, 3))
	fmt.Println(Apply(Id[string], 3)) // ERROR "cannot use 3 \(untyped int constant\) as string value in argument to Apply"
	var m = Map // ERROR "cannot use generic function Map without instantiation"
	_ = m
}
