// phase: 07

// defer and recover inside generic functions, and deferred calls of generic functions with explicit and inferred type arguments.
package main

import "fmt"

func Log[T any](tag string, v T) { fmt.Println("log", tag, v) }

func Cleanup[T any](v T) { fmt.Println("cleanup", v) }

func Safe[T any](f func() T) (res T, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("recovered: %v", r)
		}
	}()
	return f(), nil
}

func Order[T any](a, b T) {
	defer Log("first", a)
	defer Log[T]("second", b)
	defer Cleanup(fmt.Sprint(a, b))
	defer func() { Log("closure", a) }()
}

func Named[T ~int](v T) (r T) {
	defer func() { r *= 2 }()
	defer Cleanup(v)
	return v + 1
}

func Rethrow[T any](v T) {
	defer func() {
		r := recover()
		fmt.Println("caught", r)
		panic(fmt.Sprint("again ", v))
	}()
	panic(v)
}

func Wrapper[T any](v T) (msg string) {
	defer func() { msg = fmt.Sprint(recover()) }()
	Rethrow(v)
	return "unreachable"
}

func main() {
	fmt.Println(Safe(func() int { return 5 }))
	fmt.Println(Safe(func() string { panic("boom") }))
	fmt.Println(Safe(func() []int { var s []int; return s[:1] }))
	Order(1, 2)
	Order("x", "y")
	fmt.Println(Named(20))
	fmt.Println(Wrapper(1.5), Wrapper("s"))
}

// Output:
// 5 <nil>
//  recovered: boom
// [] recovered: runtime error: slice bounds out of range [:1] with capacity 0
// log closure 1
// cleanup 1 2
// log second 2
// log first 1
// log closure x
// cleanup xy
// log second y
// log first x
// cleanup 20
// 42
// caught 1.5
// caught s
// again 1.5 again s
