// phase: 07

// Mistakes in the declaration of generic functions: constraints, type parameter lists, names.
package main

type Number interface{ ~int | ~float64 }

func Dup[T any, T any](x T) {} // ERROR "T redeclared in this block"

func BadConstraint[T int](x T) {}

func Core[T int | string](x T) T { return x + x }

func Union[T int | []int](x T) {
	_ = len(x) // ERROR "invalid argument: x \(variable of type T constrained by int \| \[\]int\) for built-in len"
}

func Method[T any](x T) {
	x.String() // ERROR "x.String undefined \(type T has no field or method String\)"
}

func Unused[T any, U any](x T) {
	var y U // ERROR "declared and not used: y"
}

func main() {}
