// phase: 07

// Recursion: a generic function calling itself with the same type arguments, and mutual recursion.
package main

import "fmt"

type Number interface{ ~int | ~int64 | ~uint8 | ~float64 }

func Fib[T Number](n T) T {
	if n < 2 {
		return n
	}
	return Fib(n-1) + Fib(n-2)
}

func Fact[T Number](n T) T {
	if n <= 1 {
		return 1
	}
	return n * Fact(n-1)
}

func Even[T ~int | ~uint8](n T) bool {
	if n == 0 {
		return true
	}
	return Odd(n - 1)
}

func Odd[T ~int | ~uint8](n T) bool {
	if n == 0 {
		return false
	}
	return Even(n - 1)
}

func Repeat[T any](n int, v T, f func(T) T) T {
	if n == 0 {
		return v
	}
	return Repeat(n-1, f(v), f)
}

func Depth[T any](n int, f func(int) T) []T {
	if n == 0 {
		return nil
	}
	return append(Depth(n-1, f), f(n))
}

func main() {
	fmt.Println(Fib(10), Fib(10.0), Fib[uint8](13), Fib[uint8](14))
	fmt.Println(Fact(10), Fact[uint8](6), Fact[uint8](7), Fact(5.0))
	fmt.Println(Even(10), Odd(10), Even[uint8](7), Odd[uint8](7))
	fmt.Println(Repeat(5, 1, func(i int) int { return i * 2 }), Repeat(3, "a", func(s string) string { return s + s }))
	fmt.Println(Depth(4, func(i int) string { return fmt.Sprint(i * i) }))
}

// Output:
// 55 55 233 121
// 3628800 208 176 120
// true false false true
// 32 aaaaaaaa
// [1 4 9 16]
