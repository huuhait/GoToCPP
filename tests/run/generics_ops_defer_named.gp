// phase: 07

// defer, recover and named results in generic functions.
package main

import (
	"errors"
	"fmt"
)

type Num interface{ ~int | ~float64 | ~int8 }

func Safe[T any](f func() T) (res T, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("recovered: %v", r)
		}
	}()
	return f(), nil
}

func Double[T Num](x T) (r T) {
	defer func() { r *= 2 }()
	return x + 1
}

func Order[T any](xs ...T) (out []T) {
	for _, x := range xs {
		defer func() { out = append(out, x) }()
	}
	return nil
}

func Div[T ~int | ~int8](a, b T) (q T, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = errors.New(fmt.Sprint(r))
		}
	}()
	q = a / b
	return
}

func Trace[T any](name string, v T) T {
	fmt.Println("enter", name, v)
	defer fmt.Println("leave", name, v)
	return v
}

func Retry[T any](n int, f func(int) (T, error)) (T, error) {
	var last error
	for i := 0; i < n; i++ {
		v, err := f(i)
		if err == nil {
			return v, nil
		}
		last = err
	}
	var zero T
	return zero, last
}

func Panicker[T any](v T) {
	defer func() { fmt.Println("deferred sees", v, recover() != nil) }()
	panic(v)
}

func main() {
	fmt.Println(Safe(func() int { return 5 }))
	fmt.Println(Safe(func() string { var m map[string]int; m["a"] = 1; return "unreachable" }))
	fmt.Println(Safe(func() []int { return []int{1}[:5] }))
	fmt.Println(Safe(func() any { var e error; return e.Error() }))
	fmt.Println(Double(4), Double(2.5), Double(int8(63)))
	fmt.Println(Order(1, 2, 3), Order("a"), Order[int]())
	fmt.Println(Div(7, 2))
	fmt.Println(Div(int8(-128), 0))
	fmt.Println(Trace("a", 1), Trace("b", "two"))
	fmt.Println(Retry(3, func(i int) (string, error) {
		if i < 2 {
			return "", fmt.Errorf("fail %d", i)
		}
		return fmt.Sprint("ok", i), nil
	}))
	fmt.Println(Retry(2, func(i int) (int, error) { return 0, fmt.Errorf("always %d", i) }))
	Panicker(7)
}

// Output:
// 5 <nil>
//  recovered: assignment to entry in nil map
// [] recovered: runtime error: slice bounds out of range [:5] with capacity 1
// <nil> recovered: runtime error: invalid memory address or nil pointer dereference
// 10 7 -128
// [3 2 1] [a] []
// 3 <nil>
// 0 runtime error: integer divide by zero
// enter a 1
// leave a 1
// enter b two
// leave b two
// 1 two
// ok2 <nil>
// 0 always 1
// deferred sees 7 true
