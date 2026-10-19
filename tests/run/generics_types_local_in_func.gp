// phase: 07

// Generic types declared at package level but instantiated inside generic functions with the function's type parameters.
package main

import "fmt"

type Box[T any] struct{ v T }

func (b Box[T]) Get() T { return b.v }

type Pair[A, B any] struct {
	A A
	B B
}

func Wrap[T any](v T) Box[T]                  { return Box[T]{v} }
func WrapTwice[T any](v T) Box[Box[T]]        { return Box[Box[T]]{Box[T]{v}} }
func Both[T, U any](t T, u U) Pair[Box[T], Box[U]] { return Pair[Box[T], Box[U]]{Box[T]{t}, Box[U]{u}} }

func Describe[T any](v T) string {
	var b Box[T]
	var bs []Box[T]
	m := map[string]Box[[]T]{}
	return fmt.Sprintf("%T %T %T %v", b, bs, m, b.Get())
}

func Outer[T any](v T) string { return Inner(Wrap(v)) }

func Inner[U any](u U) string { return fmt.Sprintf("%T:%v", u, u) }

func main() {
	fmt.Println(Wrap(1), WrapTwice("s"), Both(1, 2.5))
	fmt.Println(Describe(0), Describe("x"), Describe([]bool{}))
	fmt.Println(Outer(3), Outer(Box[string]{"nested"}))
	fmt.Printf("%T %T\n", WrapTwice(1.5), Both("a", []int{1}))
}

// Output:
// {1} {{s}} {{1} {2.5}}
// main.Box[int] []main.Box[int] map[string]main.Box[[]int] 0 main.Box[string] []main.Box[string] map[string]main.Box[[]string]  main.Box[[]bool] []main.Box[[]bool] map[string]main.Box[[][]bool] []
// main.Box[int]:{3} main.Box[main.Box[string]]:{{nested}}
// main.Box[main.Box[float64]] main.Pair[main.Box[string],main.Box[[]int]]
