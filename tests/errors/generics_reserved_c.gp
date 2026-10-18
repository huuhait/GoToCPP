// phase: 07

// C is reserved for C's built-in types, also as the name of a type parameter.
package main

func Compose[A, B, C any](f func(A) B, g func(B) C) func(A) C { // ERROR "cannot declare C - C is reserved for C's built-in types"
	return func(a A) C { return g(f(a)) }
}

func main() {}
