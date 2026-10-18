// phase: 07

// Min and Max with an Ordered constraint written in the program.
package main

import "fmt"

type Ordered interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 | ~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 |
		~float32 | ~float64 | ~string
}

type Level uint8

func Max[T Ordered](a, b T) T {
	if a > b {
		return a
	}
	return b
}

func Min[T Ordered](a, b T) T {
	if a < b {
		return a
	}
	return b
}

func MaxOf[T Ordered](first T, rest ...T) T {
	m := first
	for _, v := range rest {
		m = Max(m, v)
	}
	return m
}

func main() {
	fmt.Println(Max(3, 7), Min(3, 7))
	fmt.Println(Max("apple", "banana"), Min("apple", "banana"))
	fmt.Println(Max(2.5, 1.5), Min(-0.5, 0.5))
	fmt.Println(Max[int8](5, -3), Max[uint8](200, 100))
	fmt.Println(Max(Level(3), Level(9)), Min(Level(3), Level(9)))
	fmt.Println(MaxOf(1), MaxOf(4, 9, 2), MaxOf("x", "yy", "z"))
	fmt.Println(Max(1<<40, 5), Max('a', 'b'))
	fmt.Printf("%T %T %T\n", Max(1, 2), Max(1.5, 2), Max('a', 'b'))
}

// Output:
// 7 3
// banana apple
// 2.5 -0.5
// 5 200
// 9 3
// 1 9 z
// 1099511627776 98
// int float64 int32
