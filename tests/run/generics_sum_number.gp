// phase: 07

// A constraint with a type set: Sum over ints, floats and a defined type.
package main

import "fmt"

type Number interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 | ~float32 | ~float64
}

type Celsius float64
type Count int8

func Sum[T Number](xs []T) T {
	var s T
	for _, x := range xs {
		s += x
	}
	return s
}

func Mean[T Number](xs []T) float64 {
	return float64(Sum(xs)) / float64(len(xs))
}

func main() {
	fmt.Println(Sum([]int{1, 2, 3}))
	fmt.Println(Sum([]float64{1.5, 2.5}))
	fmt.Println(Sum([]Celsius{36.6, 0.4}))
	fmt.Println(Sum([]Count{100, 27}), Sum([]Count{100, 28}))
	fmt.Println(Sum([]float32{0.1, 0.2}))
	fmt.Println(Sum[int](nil))
	fmt.Println(Mean([]int{1, 2}), Mean([]Celsius{1, 2, 4}))
	fmt.Printf("%T %T %T\n", Sum([]Celsius{1}), Sum([]int16{1}), Mean([]int8{1}))
}

// Output:
// 6
// 4
// 37
// 127 -128
// 0.3
// 0
// 1.5 2.3333333333333335
// main.Celsius int16 float64
