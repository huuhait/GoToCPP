// phase: 07

// Instantiation errors: wrong number of type arguments, a generic function or type used without
// instantiation, failed inference, and arguments that disagree. The messages are those of gc
// (types2) and go/types.
package main

import "fmt"

type Stack[T any] struct{ items []T }

type Pair[K comparable, V any] struct {
	Key K
	Val V
}

func One[T any](x T) T { return x }

func Two[T, U any](x T) (T, U) { var u U; return x, u }

func Same[T any](a, b T) bool { return true }

func Apply[T any](f func(T) T, x T) T { return f(x) }

func Make[T any]() T { var z T; return z }

func main() {
	var s Stack // ERROR "cannot use generic type Stack\[T any\] without instantiation"
	_ = s
	var p Pair[string] // ERROR "not enough type arguments for type Pair: have 1, want 2"
	_ = p
	var q Pair[string, int, bool] // ERROR "too many type arguments for type Pair: have 3, want 2"
	_ = q
	f := One // ERROR "cannot use generic function One without instantiation"
	_ = f
	_ = One[int, string](1) // ERROR "got 2 type arguments but want 1"
	_ = Make()              // ERROR "in call to Make, cannot infer T"
	_, _ = Two(1)           // ERROR "in call to Two, cannot infer U"
	_ = Same(1, "a")        // ERROR "mismatched types untyped int and untyped string"
	_ = Same(1, 2.5)
	var i int
	var s2 string
	_ = Same(i, s2)                                            // ERROR "type string of s2 does not match inferred type int for T"
	_ = Apply(func(s string) string { return s }, 5)           // ERROR "cannot use 5 \(untyped int constant\) as string value in argument to Apply"
	_ = Apply(func(a, b int) int { return a }, 5)              // ERROR "type func\(a int, b int\) int of .* does not match"
	_ = Pair[[]int, string]{}                                  // ERROR "\[\]int does not satisfy comparable"
	_ = Pair[func(), int]{}                                    // ERROR "func\(\) does not satisfy comparable"
	var _ Stack[int] = Stack[string]{}                         // ERROR "cannot use Stack\[string\]\{\} \(value of struct type Stack\[string\]\) as Stack\[int\] value in variable declaration"
	fmt.Println(One[int]("x"))                                 // ERROR "cannot use \x22x\x22 \(untyped string constant\) as int value in argument to One\[int\]"
	var g func(int) int = One[string]                          // ERROR "cannot use One\[string\] \(value of type func\(x string\) string\) as func\(int\) int value in variable declaration"
	_ = g
}
