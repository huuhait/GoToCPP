// phase: 07

// Go's order of evaluation in generic code: calls, assignments and function arguments with side effects.
package main

import "fmt"

var trace []string

func Note[T any](tag string, v T) T {
	trace = append(trace, tag)
	return v
}

func Add[T ~int | ~string](a, b T) T { return a + b }

func Three[T any](a, b, c T) []T { return []T{a, b, c} }

func Swap[T any](xs []T, i, j int) {
	xs[Note("i", i)], xs[Note("j", j)] = xs[j], xs[i]
}

func Pick[T any](ok bool, a, b func() T) T {
	if ok {
		return a()
	}
	return b()
}

func main() {
	fmt.Println(Add(Note("a", 1), Note("b", 2)), Add(Note("c", "x"), Note("d", "y")))
	fmt.Println(Three(Note("e", 1), Note("f", 2), Note("g", 3)))
	xs := []string{"a", "b", "c"}
	Swap(xs, 0, 2)
	fmt.Println(xs)
	fmt.Println(Pick(true, func() int { return Note("t", 1) }, func() int { return Note("f", 2) }), Pick(false, func() string { return Note("t2", "x") }, func() string { return Note("f2", "y") }))
	fmt.Println(trace)
	var m = map[string]int{}
	m[Note("k1", "a")] += Note("v1", 1)
	m[Note("k2", "a")] += Note("v2", 2)
	fmt.Println(m, trace[len(trace)-4:])
}

// Output:
// 3 xy
// [1 2 3]
// [c b a]
// 1 y
// [a b c d e f g i j t f2]
// map[a:3] [k1 v1 k2 v2]
