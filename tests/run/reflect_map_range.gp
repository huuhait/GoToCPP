// phase: 09

package main

import (
	"fmt"
	"reflect"
)

type Pt struct{ X, Y int }

func main() {
	m := map[string]int{"a": 1, "b": 2, "c": 3, "d": 4}
	it := reflect.ValueOf(m).MapRange()
	sum, keys := 0, 0
	for it.Next() {
		sum += int(it.Value().Int())
		keys += len(it.Key().String())
	}
	fmt.Println(sum, keys)
	mp := map[Pt][]string{{1, 2}: {"x"}, {3, 4}: {"y", "z"}}
	total := 0
	for it := reflect.ValueOf(mp).MapRange(); it.Next(); {
		total += it.Key().Field(0).Interface().(int) + it.Value().Len()
	}
	fmt.Println(total)
	var nilmap map[int]int
	n := 0
	for it := reflect.ValueOf(nilmap).MapRange(); it.Next(); {
		n++
	}
	fmt.Println(n)
	it2 := reflect.ValueOf(map[int]int{1: 10}).MapRange()
	func() {
		defer func() { fmt.Println("recovered:", recover()) }()
		it2.Key()
	}()
	fmt.Println(it2.Next(), it2.Key(), it2.Value(), it2.Next())
	func() {
		defer func() { fmt.Println("recovered:", recover()) }()
		it2.Key()
	}()
	func() {
		defer func() { fmt.Println("recovered:", recover()) }()
		it2.Next()
	}()
	it2.Reset(reflect.ValueOf(map[int]int{5: 6}))
	fmt.Println(it2.Next(), it2.Key(), it2.Value())
	func() {
		defer func() { fmt.Println("recovered:", recover()) }()
		reflect.ValueOf(1).MapRange()
	}()
}

// Output:
// 10 4
// 7
// 0
// recovered: MapIter.Key called before Next
// true 1 10 false
// recovered: MapIter.Key called on exhausted iterator
// recovered: MapIter.Next called on exhausted iterator
// true 5 6
// recovered: reflect: call of reflect.Value.MapRange on int Value
