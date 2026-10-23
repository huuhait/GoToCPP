// phase: 07

// Two functions can declare a generic type with the same name: they are two types, also with the same type arguments
// (checked by converting to any), and a generic type declared in a block hides a package-level one.
package main

import "fmt"

type Pair[K comparable, V any] struct {
	k K
	v V
}

func one() any {
	type Pair[K comparable, V any] struct {
		k K
		v V
	}
	return Pair[int, int]{1, 2}
}

func two() any {
	type Pair[K comparable, V any] struct {
		k K
		v V
	}
	return Pair[int, int]{1, 2}
}

func three() any { return Pair[int, int]{1, 2} }

func block() {
	type Pair[T any] struct{ x T }
	fmt.Println(Pair[int]{4})
	{
		type Pair[T, U any] struct {
			x T
			y U
		}
		fmt.Println(Pair[int, string]{5, "five"})
	}
}

func main() {
	a, b, c := one(), two(), three()
	fmt.Println(a == b, a == c, b == c, a == one(), b == two(), c == three())
	fmt.Printf("%T %T %T\n", a, b, c)
	_, isPackage := a.(Pair[int, int])
	_, isPackageC := c.(Pair[int, int])
	fmt.Println(isPackage, isPackageC)
	block()
}

// Output:
// false false false true true true
// main.Pair[int,int] main.Pair[int,int] main.Pair[int,int]
// false true
// {4}
// {5 five}
