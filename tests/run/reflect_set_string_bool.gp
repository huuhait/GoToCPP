// phase: 09

package main

import (
	"fmt"
	"reflect"
)

type Name string
type Flag bool

func main() {
	var s string
	var n Name
	var b bool
	var f Flag
	reflect.ValueOf(&s).Elem().SetString("hello")
	reflect.ValueOf(&n).Elem().SetString("named")
	reflect.ValueOf(&b).Elem().SetBool(true)
	reflect.ValueOf(&f).Elem().SetBool(true)
	fmt.Println(s, n, b, f)
	sv := reflect.ValueOf(&s).Elem()
	sv.SetString(sv.String() + " world")
	fmt.Println(s, sv.Len())
	long := make([]byte, 1000)
	for i := range long {
		long[i] = 'a' + byte(i%26)
	}
	sv.SetString(string(long))
	fmt.Println(len(s), s[:30])
	arr := []string{"x", "y"}
	reflect.ValueOf(arr).Index(0).SetString("z")
	fmt.Println(arr)
}

// Output:
// hello named true true
// hello world 11
// 1000 abcdefghijklmnopqrstuvwxyzabcd
// [z y]
