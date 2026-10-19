// phase: 09

package main

import (
	"fmt"
	"reflect"
)

type inner struct{ a, b int }
type Outer struct {
	Pub  string
	priv inner
	P    *inner
	any  any
	s    []string
	m    map[string]int
}
type Embed struct {
	inner
	N int
}
type Other struct{ Pub string }

func main() {
	o1 := Outer{"x", inner{1, 2}, &inner{3, 4}, 5, []string{"a"}, map[string]int{"k": 1}}
	o2 := Outer{"x", inner{1, 2}, &inner{3, 4}, 5, []string{"a"}, map[string]int{"k": 1}}
	fmt.Println(reflect.DeepEqual(o1, o2))
	o2.priv.b = 9
	fmt.Println(reflect.DeepEqual(o1, o2))
	o2.priv.b = 2
	o2.P.a = 0
	fmt.Println(reflect.DeepEqual(o1, o2))
	o2.P.a = 3
	o2.any = "5"
	fmt.Println(reflect.DeepEqual(o1, o2))
	o2.any = 5
	o2.s = []string{"a", "b"}
	fmt.Println(reflect.DeepEqual(o1, o2))
	o2.s = []string{"a"}
	o2.m["k"] = 2
	fmt.Println(reflect.DeepEqual(o1, o2))
	fmt.Println(reflect.DeepEqual(Embed{inner{1, 2}, 3}, Embed{inner{1, 2}, 3}), reflect.DeepEqual(Embed{inner{1, 2}, 3}, Embed{inner{1, 9}, 3}))
	fmt.Println(reflect.DeepEqual(Outer{}, Outer{}), reflect.DeepEqual(Outer{s: []string{}}, Outer{}), reflect.DeepEqual(Other{"a"}, Outer{Pub: "a"}))
	fmt.Println(reflect.DeepEqual(struct{ A int }{1}, struct{ A int }{1}), reflect.DeepEqual(struct{ A int }{1}, struct{ B int }{1}))
}

// Output:
// true
// false
// false
// false
// false
// false
// true false
// true false false
// true false
