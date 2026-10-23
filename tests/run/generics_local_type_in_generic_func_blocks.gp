// phase: 07

// Two generic types of the same name declared in one generic function, one in an inner block, are two types (the place of
// the declaration is part of the identity), as are two plain local types of the same name.
package main

import "fmt"

func G[T any](v T) {
	type Box[U any] struct{ u U }
	type Plain struct{ t T }
	o := any(Box[int]{1})
	p := any(Plain{v})
	{
		type Box[U any] struct{ u U }
		type Plain struct{ t T }
		i := any(Box[int]{1})
		q := any(Plain{v})
		fmt.Println(o == i, p == q, o == any(Box[int]{1}))
		fmt.Printf("%T %T\n", o, i)
	}
}

func main() {
	G(1)
	G("s")
}

// Output:
// false false false
// main.Box[int;int] main.Box[int;int]
// false false false
// main.Box[string;int] main.Box[string;int]
