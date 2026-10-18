// phase: 07

// Names in generic code: type parameters shadow package names, locals shadow type parameters' names, and parameters are named like types.
package main

import "fmt"

type T struct{ n int }
type Item struct{ v string }

func Shadow[T any](v T) string {
	return fmt.Sprintf("%v/%T", v, v)
}

func Local[X any](x X) string {
	T := fmt.Sprint(x)
	return T + "!"
}

func Param[P any](Item P) P { return Item }

func Names[fmt any](v fmt) string { return "no fmt here" }

func Same[A, B any](a A, b B) string {
	var A_ = a
	var B_ = b
	return fmt.Sprint(A_, B_)
}

func Inner[S ~[]E, E any](s S) int {
	n := 0
	for range s {
		E := n
		n = E + 1
	}
	return n
}

func Closure[V any](v V) func() V {
	V2 := v
	return func() V { return V2 }
}

func main() {
	fmt.Println(Shadow(1), Shadow("s"), Shadow(T{2}), Shadow(Item{"i"}))
	fmt.Println(Local(5), Local(T{3}), Param(7), Param("p"), Names(1), Same(1, "a"))
	fmt.Println(Inner([]int{1, 2, 3}), Closure(Item{"c"})(), Closure[T](T{4})())
	fmt.Println(T{1}, Item{"x"})
}

// Output:
// 1/int s/string {2}/main.T {i}/main.Item
// 5! {3}! 7 p no fmt here 1a
// 3 {c} {4}
// {1} {x}
