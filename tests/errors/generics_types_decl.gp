// phase: 07

// Declaring methods of generic types wrongly: the receiver's type parameters must match the type's.
package main

type Stack[T any] struct{ items []T }

func (s *Stack[T, U]) Bad() {} // ERROR "receiver declares 2 type parameters, but receiver base type declares 1"

func (s Stack) NoArgs() {} // ERROR "cannot use generic type Stack\[T any\] without instantiation"

func (s *Stack[int]) Shadow() int { return 0 } // ERROR "cannot use 0 \(untyped int constant\) as int/\* type parameter \*/ value in return statement"

type Pair[K comparable, V any] struct {
	Key K
	Val V
}

func (p Pair[K, V]) Swap() Pair[V, K] { return Pair[V, K]{p.Val, p.Key} } // ERROR "V does not satisfy comparable"

type Bad[T any] struct{ next Bad[T] } // ERROR "invalid recursive type: Bad refers to itself"

func main() {}
