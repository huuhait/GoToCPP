// phase: 07

// Type arguments that cannot be inferred, are inconsistent, or are too many.
package main

import "fmt"

type Number interface{ ~int | ~float64 }

func Sum[T Number](xs ...T) T {
	var s T
	return s
}

func Zero[T any]() T {
	var z T
	return z
}

func Pair[A, B any](a A, b B) (A, B) { return a, b }

func Map[T, U any](xs []T, f func(T) U) []U { return nil }

func Id[T any](x T) T { return x }

func main() {
	fmt.Println(Zero())                            // ERROR "in call to Zero, cannot infer T"
	fmt.Println(Pair[int, string, bool](1, "a"))   // ERROR "got 3 type arguments but want 2"
	fmt.Println(Sum(1, 2.5, "x"))                  // ERROR "mismatched types untyped float and untyped string \(cannot infer T\)"
	fmt.Println(Sum(1, "x"))                       // ERROR "mismatched types untyped int and untyped string \(cannot infer T\)"
	fmt.Println(Map([]int{1}, nil))                // ERROR "in call to Map, cannot infer U"
	var x int = Id("s")                            // ERROR "cannot use Id\(\x22s\x22\) \(value of type string\) as int value in variable declaration"
	fmt.Println(x, Sum[int, int](1))               // ERROR "got 2 type arguments but want 1"
	_, _ = Pair[int](1, 2)
	_, _ = Pair[int, int](1, "a") // ERROR "cannot use \x22a\x22 \(untyped string constant\) as int value in argument to Pair\[int, int\]"
	var a int
	var b string
	_, _ = Pair[string](a, b) // ERROR "cannot use a \(variable of type int\) as string value in argument to Pair\[string\]"
}
