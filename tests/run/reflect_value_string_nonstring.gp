// phase: 09

package main

import (
	"fmt"
	"reflect"
)

type Pt struct{ X int }

func main() {
	vals := []any{1, 2.5, true, []int{1}, map[string]int{}, Pt{}, &Pt{}, "str", [2]int{}, 'x', func() {}}
	for _, x := range vals {
		fmt.Println(reflect.ValueOf(x).String())
	}
	fmt.Println(reflect.Value{}.String())
	fmt.Println(reflect.ValueOf(&struct{ F any }{}).Elem().Field(0).String())
}

// Output:
// <int Value>
// <float64 Value>
// <bool Value>
// <[]int Value>
// <map[string]int Value>
// <main.Pt Value>
// <*main.Pt Value>
// str
// <[2]int Value>
// <int32 Value>
// <func() Value>
// <invalid Value>
// <interface {} Value>
