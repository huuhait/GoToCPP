// phase: 09

package main

import (
	"fmt"
	"reflect"
)

type Pt struct{ X, Y int }

func main() {
	s := []Pt{{1, 2}, {3, 4}, {5, 6}}
	v := reflect.ValueOf(s)
	v.Index(0).Set(reflect.ValueOf(Pt{9, 9}))
	v.Index(1).Field(0).SetInt(-3)
	fmt.Println(s)
	ps := []*Pt{{1, 1}, nil}
	pv := reflect.ValueOf(ps)
	pv.Index(1).Set(reflect.ValueOf(&Pt{7, 7}))
	pv.Index(0).Elem().Field(1).SetInt(8)
	fmt.Println(*ps[0], *ps[1])
	ss := [][]int{{1}, {2}}
	sv := reflect.ValueOf(ss)
	sv.Index(0).Set(reflect.ValueOf([]int{10, 20}))
	sv.Index(1).Index(0).SetInt(-2)
	fmt.Println(ss)
	ms := []map[string]int{{"a": 1}, nil}
	mv := reflect.ValueOf(ms)
	mv.Index(1).Set(reflect.ValueOf(map[string]int{"b": 2}))
	mv.Index(0).SetMapIndex(reflect.ValueOf("a"), reflect.ValueOf(100))
	fmt.Println(ms)
	strs := []string{"a", "b"}
	alias := reflect.ValueOf(strs)
	alias.Index(0).SetString("shared")
	fmt.Println(strs, alias.Index(0).CanSet(), alias.Index(0).CanAddr())
}

// Output:
// [{9 9} {-3 4} {5 6}]
// {1 8} {7 7}
// [[10 20] [-2]]
// [map[a:100] map[b:2]]
// [shared b] true true
