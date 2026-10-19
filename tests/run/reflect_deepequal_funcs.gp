// phase: 09

package main

import (
	"fmt"
	"reflect"
)

type S struct {
	F func()
	N int
}

func f() {}

func main() {
	var nf1, nf2 func()
	fmt.Println(reflect.DeepEqual(nf1, nf2), reflect.DeepEqual(f, f), reflect.DeepEqual(f, nf1), reflect.DeepEqual(func() {}, func() {}))
	fmt.Println(reflect.DeepEqual(S{nil, 1}, S{nil, 1}), reflect.DeepEqual(S{f, 1}, S{f, 1}), reflect.DeepEqual(S{nil, 1}, S{f, 1}))
	fmt.Println(reflect.DeepEqual([]func(){nil}, []func(){nil}), reflect.DeepEqual([]func(){f}, []func(){f}))
	fmt.Println(reflect.DeepEqual(map[string]func(){"a": nil}, map[string]func(){"a": nil}), reflect.DeepEqual(map[string]func(){"a": f}, map[string]func(){"a": f}))
	fp := &S{F: f}
	fmt.Println(reflect.DeepEqual(fp, fp), reflect.DeepEqual(fp, &S{F: f}))
}

// Output:
// true false false false
// true false false
// true false
// true false
// true false
