// phase: 09

package main

import (
	"fmt"
	"reflect"
)

type S struct{ I any }
type T struct{ A [2]any }

func main() {
	v := reflect.ValueOf
	vals := []any{1, "s", []int{}, map[int]int{}, func() {}, S{}, S{1}, S{[]int{}}, T{}, T{[2]any{1, []int{}}}, [2]int{}, [2][]int{}, &S{}, make(chan int), struct{}{}}
	for _, x := range vals {
		fmt.Printf("%T %v\n", x, v(x).Comparable())
	}
	fmt.Println(reflect.Value{}.Comparable())
	var e error
	fmt.Println(v(&e).Elem().Comparable())
}

// Output:
// int true
// string true
// []int false
// map[int]int false
// func() false
// main.S true
// main.S true
// main.S false
// main.T true
// main.T false
// [2]int true
// [2][]int false
// *main.S true
// chan int true
// struct {} true
// false
// true
