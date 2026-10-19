// phase: 07

// A generic type over a slice with methods; the type survives slicing, append and conversion.
package main

import "fmt"

type Vec[T any] []T

func (v Vec[T]) Len() int { return len(v) }

func (v Vec[T]) Map(f func(T) T) Vec[T] {
	out := make(Vec[T], len(v))
	for i, x := range v {
		out[i] = f(x)
	}
	return out
}

func (v *Vec[T]) Push(x T) { *v = append(*v, x) }

func (v Vec[T]) Last() (T, bool) {
	if len(v) == 0 {
		var z T
		return z, false
	}
	return v[len(v)-1], true
}

type Numbers[T ~int | ~float64] []T

func (n Numbers[T]) Sum() T {
	var s T
	for _, x := range n {
		s += x
	}
	return s
}

func main() {
	var v Vec[int]
	v.Push(1)
	v.Push(2)
	v.Push(3)
	w := v.Map(func(i int) int { return i * i })
	fmt.Println(v, w, w.Len())
	fmt.Println(v.Last())
	fmt.Println(Vec[string](nil).Last())
	fmt.Printf("%T %T %T\n", v, w[1:], append(w, 4))
	fmt.Println([]int(w), Vec[int]([]int{9}).Len())
	fmt.Println(Numbers[int]{1, 2, 3}.Sum(), Numbers[float64]{0.5, 0.25}.Sum())
}

// Output:
// [1 2 3] [1 4 9] 3
// 3 true
//  false
// main.Vec[int] main.Vec[int] main.Vec[int]
// [1 4 9] 1
// 6 0.75
