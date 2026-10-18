// phase: 07

// Constants in generic code take the type of the type argument, and are checked in each instance.
package main

import "fmt"

type Num interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 | ~float32 | ~float64
}

type Small int8

const K = 100

func Plus1[T Num](x T) T { return x + 1 }

func Scale[T Num](x T) T { return x * 2 }

func Half[T ~float32 | ~float64](x T) T { return x * 0.5 }

func Cmp[T Num](x T) bool { return x > 50 }

func Konst[T Num](x T) T { return x + K }

func Typed[T Num]() T {
	var z T = 3
	z += 4
	return z
}

func Lit[T Num]() []T { return []T{1, 2, 3} }

func Map[T Num]() map[string]T { return map[string]T{"a": 1, "b": 2} }

func Arr[T Num]() [3]T { return [3]T{7, 8, 9} }

func main() {
	fmt.Println(Plus1(1), Plus1(1.5), Plus1[int8](127), Plus1(Small(-1)), Plus1[float32](0.1))
	fmt.Println(Scale[int8](100), Scale(21), Scale(0.25), Scale[Small](64))
	fmt.Println(Half(5.0), Half[float32](1), Half(float32(3)))
	fmt.Println(Cmp[int8](51), Cmp(50), Cmp(50.5), Cmp(Small(100)))
	fmt.Println(Konst[int8](27), Konst[int8](28), Konst(1), Konst(0.5), Konst[int16](-100))
	fmt.Println(Typed[int8](), Typed[float64](), Typed[Small]())
	fmt.Println(Lit[int8](), Lit[float32](), Map[Small](), Arr[float64]())
}

// Output:
// 2 2.5 -128 0 1.1
// -56 42 0.5 -128
// 2.5 0.5 1.5
// true false true true
// 127 -128 101 100.5 0
// 7 7 7
// [1 2 3] [1 2 3] map[a:1 b:2] [7 8 9]
