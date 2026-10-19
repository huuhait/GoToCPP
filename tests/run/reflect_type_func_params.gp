// phase: 09

package main

import (
	"fmt"
	"reflect"
)

type T struct{}

func (T) M(a int, b ...string) (bool, error) { return false, nil }

func f(a int, b string, c ...float64) (int, error) { return 0, nil }

func main() {
	t := reflect.TypeOf(f)
	fmt.Println(t.NumIn(), t.NumOut(), t.IsVariadic())
	for i := 0; i < t.NumIn(); i++ {
		fmt.Println("in", i, t.In(i))
	}
	for i := 0; i < t.NumOut(); i++ {
		fmt.Println("out", i, t.Out(i))
	}
	g := reflect.TypeOf(func(x []int) {})
	fmt.Println(g.NumIn(), g.NumOut(), g.IsVariadic(), g.In(0))
	h := reflect.TypeOf(func(...any) {})
	fmt.Println(h.IsVariadic(), h.In(0), h.In(0).Elem())
	k := reflect.TypeOf(func(func(int, ...string) bool) {})
	fmt.Println(k.In(0), k.In(0).IsVariadic(), k.In(0).In(1))
}

// Output:
// 3 2 true
// in 0 int
// in 1 string
// in 2 []float64
// out 0 int
// out 1 error
// 1 0 false []int
// true []interface {} interface {}
// func(int, ...string) bool true []string
