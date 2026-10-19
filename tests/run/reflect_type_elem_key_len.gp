// phase: 09

package main

import (
	"fmt"
	"reflect"
)

type P struct{ X int }

func main() {
	fmt.Println(reflect.TypeOf([]int{}).Elem(), reflect.TypeOf([3]string{}).Elem(), reflect.TypeOf([3]string{}).Len())
	fmt.Println(reflect.TypeOf(map[string]P{}).Key(), reflect.TypeOf(map[string]P{}).Elem())
	fmt.Println(reflect.TypeOf(&P{}).Elem(), reflect.TypeOf(make(chan []int)).Elem())
	fmt.Println(reflect.TypeOf([0]int{}).Len(), reflect.TypeOf([1 << 20]byte{}).Len())
	fmt.Println(reflect.TypeOf(new(*P)).Elem().Elem().Field(0).Name)
	t := reflect.TypeOf(map[[2]int][]*P{})
	fmt.Println(t.Key(), t.Key().Len(), t.Elem().Elem().Elem())
}

// Output:
// int string 3
// string main.P
// main.P []int
// 0 1048576
// X
// [2]int 2 main.P
