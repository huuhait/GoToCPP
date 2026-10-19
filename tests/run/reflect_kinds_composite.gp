// phase: 09

package main

import (
	"fmt"
	"reflect"
	"unsafe"
)

type S struct{ A int }
type I interface{ M() }

func main() {
	var ip I
	var up unsafe.Pointer
	vals := []any{
		[]int{}, [3]int{}, map[string]int{}, new(int), S{}, func() {}, &ip, make(chan int), make(<-chan string), make(chan<- bool), up, &S{},
		[][]string{}, map[S][]*S{}, [0]int{}, struct{}{}, func(int) string { return "" },
	}
	for _, v := range vals {
		t := reflect.TypeOf(v)
		fmt.Printf("%-28s %-14v %q\n", t, t.Kind(), t.Name())
	}
	fmt.Println(reflect.TypeOf(&ip).Elem().Kind(), reflect.TypeOf(&ip).Elem())
}

// Output:
// []int                        slice          ""
// [3]int                       array          ""
// map[string]int               map            ""
// *int                         ptr            ""
// main.S                       struct         "S"
// func()                       func           ""
// *main.I                      ptr            ""
// chan int                     chan           ""
// <-chan string                chan           ""
// chan<- bool                  chan           ""
// unsafe.Pointer               unsafe.Pointer "Pointer"
// *main.S                      ptr            ""
// [][]string                   slice          ""
// map[main.S][]*main.S         map            ""
// [0]int                       array          ""
// struct {}                    struct         ""
// func(int) string             func           ""
// interface main.I
