// phase: 09

package main

import (
	"fmt"
	"reflect"
)

type A struct{ X int }
type B struct{ S []int }
type Cc struct{ I any }
type D struct{ M map[string]int }
type E struct{ A A }
type F struct{ f func() }
type G struct{ arr [2]B }

func main() {
	vals := []any{
		1, "s", 1.5, true, A{}, B{}, Cc{}, D{}, E{}, F{}, G{}, [2]int{}, [2][]int{}, []int{}, map[int]int{}, func() {},
		&A{}, make(chan int), complex(1, 2), [0]func(){}, struct{}{}, new(any),
	}
	for _, v := range vals {
		t := reflect.TypeOf(v)
		fmt.Println(t, t.Comparable())
	}
	var e error
	fmt.Println(reflect.TypeOf(&e).Elem().Comparable())
}

// Output:
// int true
// string true
// float64 true
// bool true
// main.A true
// main.B false
// main.Cc true
// main.D false
// main.E true
// main.F false
// main.G false
// [2]int true
// [2][]int false
// []int false
// map[int]int false
// func() false
// *main.A true
// chan int true
// complex128 true
// [0]func() false
// struct {} true
// *interface {} true
// true
