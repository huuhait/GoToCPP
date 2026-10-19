// phase: 07

// Types declared in generic functions and in methods of generic types are types of each instance: main.local[int,string] (also when they do not mention a type parameter), distinct per function and per list of type arguments.
package main

import "fmt"

func F[T any, U any](v T) any {
	type local struct{ x T }
	type plain struct{ n int }
	type e struct{ local }
	fmt.Printf("%T %T %T\n", local{v}, plain{}, e{})
	fmt.Printf("%+v\n", e{local{v}})
	return local{v}
}

func H[T any](v T) any {
	type local struct{ x T }
	return local{v}
}

func P[T any](v T) any {
	type local struct{ x T }
	return local{v}
}

func Q[T any](v T) any {
	type local struct{ x T }
	return local{v}
}

func G() any {
	type local struct{ x int }
	return local{1}
}

type B[T any] struct{ v T }

func (b B[T]) M() any {
	type in struct{ y T }
	return in{b.v}
}

func main() {
	fmt.Printf("%T\n", F[int, string](1))
	fmt.Printf("%T\n", F[[]string, map[int]bool](nil))
	fmt.Printf("%T\n", G())
	fmt.Printf("%T\n", B[int]{1}.M())
	fmt.Printf("%T %v\n", B[string]{"s"}.M(), B[string]{"s"}.M())
	a, b := F[int, int](1), H(1)
	fmt.Println(a == b, a == F[int, int](1), fmt.Sprintf("%T", a) == fmt.Sprintf("%T", b))
	p, q := P(1), Q(1)
	fmt.Println(p == q, fmt.Sprintf("%T", p) == fmt.Sprintf("%T", q), p == P(1), P("s") == p)
	_, ok := a.(interface{ Foo() })
	fmt.Println(ok)
}

// Output:
// main.local[int,string] main.plain[int,string] main.e[int,string]
// {local:{x:1}}
// main.local[int,string]
// main.local[[]string,map[int]bool] main.plain[[]string,map[int]bool] main.e[[]string,map[int]bool]
// {local:{x:[]}}
// main.local[[]string,map[int]bool]
// main.local
// main.in[int]
// main.in[string] {s}
// main.local[int,int] main.plain[int,int] main.e[int,int]
// {local:{x:1}}
// main.local[int,int] main.plain[int,int] main.e[int,int]
// {local:{x:1}}
// false true false
// false true true false
// false
