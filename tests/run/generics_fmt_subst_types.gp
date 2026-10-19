// phase: 08

// Types that mention a type parameter inside generic code: channel directions, struct tags, anonymous
// interfaces that embed a generic interface, variadic func types; the substitution must keep them.
package main

import "fmt"

type It[T any] interface{ Get() T }
type Named interface{ Name() string }

func Chans[T any]() []any {
	var r <-chan T
	var s chan<- T
	var d chan T
	return []any{r, s, d, (<-chan []T)(nil), map[string]<-chan T(nil)}
}

func Tags[T any]() []any {
	return []any{
		struct {
			X T `json:"x"`
			Y int
		}{},
		[]struct {
			A T `k:"v"`
		}{},
	}
}

func Ifaces[T any]() []any {
	var a interface {
		It[T]
		Named
	}
	var b interface{ It[[]T] }
	var c *interface {
		Set(T)
		It[T]
	}
	var f func(interface{ It[T] }) T
	return []any{&a, &b, c, f}
}

func Fn[T any]() []any {
	var a func(...T)
	var b func(a T, b ...[]T) (T, error)
	return []any{a, b}
}

type V[T any] struct {
	c <-chan T
	t struct {
		X T `a:"b"`
	}
	i interface{ It[T] }
	f func(...T)
}

func main() {
	for _, v := range Chans[int]() {
		fmt.Printf("%T\n", v)
	}
	for _, v := range Tags[string]() {
		fmt.Printf("%T\n", v)
	}
	for _, v := range Ifaces[float64]() {
		fmt.Printf("%T\n", v)
	}
	for _, v := range Fn[bool]() {
		fmt.Printf("%T\n", v)
	}
	fmt.Printf("%+v %T\n", V[int8]{}, V[int8]{}.t)
}

// Output:
// <-chan int
// chan<- int
// chan int
// <-chan []int
// map[string]<-chan int
// struct { X string "json:\"x\""; Y int }
// []struct { A string "k:\"v\"" }
// *interface { Get() float64; Name() string }
// *interface { Get() []float64 }
// *interface { Get() float64; Set(float64) }
// func(interface { Get() float64 }) float64
// func(...bool)
// func(bool, ...[]bool) (bool, error)
// {c:<nil> t:{X:0} i:<nil> f:<nil>} struct { X int8 "a:\"b\"" }
