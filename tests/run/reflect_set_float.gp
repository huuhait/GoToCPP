// phase: 09

package main

import (
	"fmt"
	"reflect"
)

type Temp float32

func main() {
	var f32 float32
	var f64 float64
	var t Temp
	v32 := reflect.ValueOf(&f32).Elem()
	v64 := reflect.ValueOf(&f64).Elem()
	vt := reflect.ValueOf(&t).Elem()
	v32.SetFloat(1.1)
	v64.SetFloat(1.1)
	vt.SetFloat(2.2)
	fmt.Println(f32, f64, t, v32.Float(), v64.Float(), vt.Float())
	fmt.Println(v32.OverflowFloat(1e39), v32.OverflowFloat(1e38), v64.OverflowFloat(1e300), v32.OverflowFloat(-1e39))
	v32.SetFloat(1e39)
	fmt.Println(f32)
	var c64 complex64
	var c128 complex128
	reflect.ValueOf(&c64).Elem().SetComplex(1.5 + 2.5i)
	reflect.ValueOf(&c128).Elem().SetComplex(-1 - 1i)
	fmt.Println(c64, c128, reflect.ValueOf(c64).OverflowComplex(complex(1e39, 0)))
}

// Output:
// 1.1 1.1 2.2 1.100000023841858 1.1 2.200000047683716
// true false false true
// +Inf
// (1.5+2.5i) (-1-1i) true
