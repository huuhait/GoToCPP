// phase: 07

// A type declared in a method of a generic type, or in a function, is a type of that declaration: two methods named alike on different generic types, two functions, and two blocks of one function that declare the same name at similar places give different types.
package main

import "fmt"

type A[T any] struct{ v T }
type B[T any] struct{ v T }

func (a A[T]) Snap() any {
	type x struct{ v T }
	return x{a.v}
}

func (b B[T]) Snap() any {
	type x struct{ v T }
	return x{b.v}
}

func (b *B[T]) Ptr() any {
	type x struct{ v T }
	return x{b.v}
}

func F1[T any](v T) any {
	type y struct{ v T }
	return y{v}
}

func F2[T any](v T) any {
	type y struct{ v T }
	return y{v}
}

func Blocks[T any](v T) (any, any) {
	if true {
		type t struct{ x T }
		return t{v}, nil
	}
	return nil, nil
}

func Blocks2[T any](v T) (any, any) {
	var r any
	{
		type t struct{ x T }
		r = t{v}
	}
	{
		type t struct{ x T }
		return r, t{v}
	}
}

func main() {
	a, b := A[int]{1}.Snap(), B[int]{1}.Snap()
	fmt.Printf("%T %T %v %v\n", a, b, a == b, a == A[int]{1}.Snap())
	p := (&B[int]{1}).Ptr()
	fmt.Println(p == b, p == (&B[int]{1}).Ptr())
	c, d := F1(1), F2(1)
	fmt.Printf("%T %T %v %v\n", c, d, c == d, c == F1(1))
	e, _ := Blocks(1)
	f, g := Blocks2(1)
	fmt.Println(e == f, f == g, e == g, f == f)
}

// Output:
// main.x[int] main.x[int] false true
// false true
// main.y[int] main.y[int] false true
// false false false true
