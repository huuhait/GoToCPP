// phase: 07
// ExitCode: 2
// Stderr: ^panic: runtime error: comparing uncomparable type \[\]int
// Stderr: (?m)^goroutine 1 \[running\]:

// == on a type parameter instantiated with an interface type compares dynamic values: slices, maps and funcs panic.
package main

import "fmt"

func Eq[T comparable](a, b T) bool { return a == b }

func Contains[T comparable](xs []T, v T) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}

func Try(f func() bool) (res string) {
	defer func() {
		if r := recover(); r != nil {
			res = fmt.Sprint("panic: ", r)
		}
	}()
	return fmt.Sprint(f())
}

type S struct{ f any }

func main() {
	fmt.Println(Try(func() bool { return Eq[any]([]int{1}, []int{1}) }))
	fmt.Println(Try(func() bool { return Eq[any](map[string]int{}, 1) }))
	fmt.Println(Try(func() bool { return Eq[any](1, []int{1}) }))
	fmt.Println(Try(func() bool { return Eq[any](func() {}, func() {}) }))
	fmt.Println(Try(func() bool { return Eq(S{[]int{1}}, S{[]int{1}}) }))
	fmt.Println(Try(func() bool { return Eq(S{1}, S{[]int{1}}) }))
	fmt.Println(Try(func() bool { return Eq([1]any{[]int{1}}, [1]any{[]int{1}}) }))
	fmt.Println(Try(func() bool { return Contains([]any{1, "a", []int{1}}, any([]int{1})) }))
	fmt.Println(Try(func() bool { return Contains([]any{1, "a"}, any(2)) }))
	fmt.Println(Try(func() bool { return Contains([]any{[]int{1}, 3}, any(3)) }))
	fmt.Println(Eq[any](nil, nil), Eq[any](1, "a"))
	fmt.Println(Eq[any]([]int{1}, []int{1}))
}

// Output:
// panic: runtime error: comparing uncomparable type []int
// false
// false
// panic: runtime error: comparing uncomparable type func()
// panic: runtime error: comparing uncomparable type []int
// false
// panic: runtime error: comparing uncomparable type []int
// panic: runtime error: comparing uncomparable type []int
// false
// true
// true false
