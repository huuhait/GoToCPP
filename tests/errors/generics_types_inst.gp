// phase: 07

// Instantiating generic types wrongly: no arguments, too many or too few, arguments that don't satisfy the constraint, and values of the wrong instance.
package main

import "fmt"

type Stack[T any] struct{ items []T }

func (s *Stack[T]) Push(v T) { s.items = append(s.items, v) }

type Pair[K comparable, V any] struct {
	Key K
	Val V
}

type Num[T ~int | ~float64] struct{ v T }

func main() {
	var a Stack // ERROR "cannot use generic type Stack\[T any\] without instantiation"
	var b Stack[int, string] // ERROR "too many type arguments for type Stack: have 2, want 1"
	var c Pair[[]int, string] // ERROR "\[\]int does not satisfy comparable"
	var d Num[string] // ERROR "string does not satisfy ~int \| ~float64 \(string missing in ~int \| ~float64\)"
	var e Pair[int] // ERROR "not enough type arguments for type Pair: have 1, want 2"
	f := Stack{} // ERROR "cannot use generic type Stack\[T any\] without instantiation"
	g := Stack[int]{items: []string{"x"}} // ERROR "cannot use \[\]string\{…\} \(value of type \[\]string\) as \[\]int value in struct literal"
	var h Stack[int]
	h.Pop() // ERROR "h.Pop undefined \(type Stack\[int\] has no field or method Pop\)"
	h.Push("s") // ERROR "cannot use \x22s\x22 \(untyped string constant\) as int value in argument to h.Push"
	var i Stack[string] = h // ERROR "cannot use h \(variable of struct type Stack\[int\]\) as Stack\[string\] value in variable declaration"
	fmt.Println(a, b, c, d, e, f, g, i)
}
