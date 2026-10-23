// phase: 07

// A generic type declared inside a function is used in the function literals of the function, and a literal can declare its
// own; the types are values of the function (returned, stored in a map or a channel-free struct, passed to generic functions).
package main

import "fmt"

func Show[T any](v T) string { return fmt.Sprintf("%v/%T", v, v) }

func Apply[T any](f func() T) T { return f() }

func main() {
	type Opt[T any] struct {
		v  T
		ok bool
	}
	some := func(s string) Opt[string] { return Opt[string]{s, true} }
	none := func() Opt[string] { return Opt[string]{} }
	fmt.Println(some("x"), none())
	fmt.Println(Show(Apply(func() Opt[int] { return Opt[int]{7, true} })))

	inner := func() any {
		type Opt[T any] struct{ x, y T }
		return Opt[int]{1, 2}
	}
	fmt.Println(Show(inner()), inner() == inner())
	fmt.Println(inner() == any(Opt[int]{1, true}))

	var fs []func() Opt[int]
	for i := range 3 {
		fs = append(fs, func() Opt[int] { return Opt[int]{i, i%2 == 0} })
	}
	for _, f := range fs {
		fmt.Print(f(), ";")
	}
	fmt.Println()
}

// Output:
// {x true} { false}
// {7 true}/main.Opt[int]
// {1 2}/main.Opt[int] true
// false
// {0 true};{1 false};{2 true};
