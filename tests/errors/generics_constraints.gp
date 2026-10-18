// phase: 07

// Operations the constraint doesn't allow, and type arguments that don't satisfy their constraint: go/types' messages.
package main

import "fmt"

type Number interface{ ~int | ~float64 }

type Stringer interface{ String() string }

type Celsius float64

func Sum[T Number](xs ...T) T {
	var s T
	for _, x := range xs {
		s += x
	}
	return s
}

func Show[T Stringer](x T) string { return x.String() }

func Add[T any](a, b T) T { return a + b } // ERROR "operator \+ not defined on a \(variable of type T constrained by any\)"

func Less[T comparable](a, b T) bool { return a < b } // ERROR "type parameter T cannot use operator <"

func Eq[T any](a, b T) bool { return a == b } // ERROR "incomparable types in type set"

func Conv[T any](x T) int { return int(x) } // ERROR "cannot convert x \(variable of type T constrained by any\) to type int"

func Len[T Number](x T) int { return len(x) } // ERROR "invalid argument: x \(variable of type T constrained by Number\) for built-in len"

func Call[T any](x T) { x.Foo() } // ERROR "x.Foo undefined \(type T has no field or method Foo\)"

func Index[T any](x T) { _ = x[0] } // ERROR "cannot index x \(variable of type T constrained by any\)"

func main() {
	fmt.Println(Sum("a", "b"))                  // ERROR "string does not satisfy Number \(string missing in ~int \| ~float64\)"
	fmt.Println(Show(5))                        // ERROR "int does not satisfy Stringer \(missing method String\)"
	fmt.Println(Sum[string]())                  // ERROR "string does not satisfy Number"
	fmt.Println(Sum[Celsius](1, 2), Sum[int8]()) // ERROR "int8 does not satisfy Number \(int8 missing in ~int \| ~float64\)"
	var n Number                                // ERROR "cannot use type Number outside a type constraint: interface contains type constraints"
	var c comparable                            // ERROR "cannot use type comparable outside a type constraint: interface is \(or embeds\) comparable"
	_, _ = n, c
}
