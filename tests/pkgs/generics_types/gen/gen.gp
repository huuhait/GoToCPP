package gen

import "fmt"

func Describe[T any](v T) string { return fmt.Sprintf("%v|%T", v, v) }

func Box[T any](v T) any { return v }

func Zero[T any]() T { var z T; return z }

func Same[T comparable](a, b T) bool { return a == b }

func TotalArea[S interface{ Area() float64 }](xs ...S) float64 {
	t := 0.0
	for _, x := range xs {
		t += x.Area()
	}
	return t
}

func Index[T comparable](xs []T, v T) int {
	for i, x := range xs {
		if x == v {
			return i
		}
	}
	return -1
}
