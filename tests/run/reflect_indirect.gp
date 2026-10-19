// phase: 09

package main

import (
	"fmt"
	"reflect"
)

type Pt struct{ X int }

func main() {
	x := 5
	p := &x
	pp := &p
	var np *int
	for _, v := range []any{x, p, pp, np, Pt{1}, &Pt{2}, "s", []int{1}} {
		r := reflect.Indirect(reflect.ValueOf(v))
		fmt.Println(r.IsValid(), r.Kind())
	}
	reflect.Indirect(reflect.ValueOf(p)).SetInt(9)
	fmt.Println(x)
	fmt.Println(reflect.Indirect(reflect.ValueOf(&Pt{7})).Field(0), reflect.Indirect(reflect.ValueOf(pp)).Kind())
}

// Output:
// true int
// true int
// true ptr
// false invalid
// true struct
// true struct
// true string
// true slice
// 9
// 7 ptr
