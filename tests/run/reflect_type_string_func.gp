// phase: 09

package main

import (
	"fmt"
	"reflect"
)

type T struct{}

func main() {
	fs := []any{
		func() {}, func(int) {}, func(int, string) bool { return false }, func(...int) {},
		func(int, ...string) (bool, error) { return false, nil }, func(a, b int) (x, y string) { return },
		func(func(int) int) func(string) {
			return nil
		}, func(*T, []T, map[string]T) (T, *T) { return T{}, nil },
		func(chan int, <-chan int, chan<- int) {}, func(any, error) {}, func(...any) {}, func([]int, [2]int) {},
	}
	for _, f := range fs {
		t := reflect.TypeOf(f)
		fmt.Println(t, t.NumIn(), t.NumOut(), t.IsVariadic())
	}
}

// Output:
// func() 0 0 false
// func(int) 1 0 false
// func(int, string) bool 2 1 false
// func(...int) 1 0 true
// func(int, ...string) (bool, error) 2 2 true
// func(int, int) (string, string) 2 2 false
// func(func(int) int) func(string) 1 1 false
// func(*main.T, []main.T, map[string]main.T) (main.T, *main.T) 3 2 false
// func(chan int, <-chan int, chan<- int) 3 0 false
// func(interface {}, error) 2 0 false
// func(...interface {}) 1 0 true
// func([]int, [2]int) 2 0 false
