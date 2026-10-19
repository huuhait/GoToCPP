// phase: 09

package main

import (
	"fmt"
	"reflect"
)

func sortKeys(keys []reflect.Value) {
	for i := range keys {
		for j := i + 1; j < len(keys); j++ {
			less := false
			switch keys[i].Kind() {
			case reflect.String:
				less = keys[j].String() < keys[i].String()
			case reflect.Int:
				less = keys[j].Int() < keys[i].Int()
			}
			if less {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}
}

func main() {
	m := map[string]int{"banana": 2, "apple": 1, "cherry": 3}
	v := reflect.ValueOf(m)
	keys := v.MapKeys()
	sortKeys(keys)
	for _, k := range keys {
		fmt.Println(k, v.MapIndex(k), v.MapIndex(k).Kind(), v.MapIndex(k).CanSet(), k.CanSet())
	}
	fmt.Println(v.MapIndex(reflect.ValueOf("zzz")).IsValid(), v.Len())
	im := map[int][]string{3: {"c"}, 1: {"a"}, 2: nil}
	iv := reflect.ValueOf(im)
	ik := iv.MapKeys()
	sortKeys(ik)
	for _, k := range ik {
		e := iv.MapIndex(k)
		fmt.Println(k.Int(), e.Len(), e.IsNil())
	}
	named := map[string]struct{ X int }{"a": {1}}
	fmt.Println(reflect.ValueOf(named).MapIndex(reflect.ValueOf("a")).Field(0))
	var empty map[string]int
	fmt.Println(len(reflect.ValueOf(empty).MapKeys()), reflect.ValueOf(empty).MapIndex(reflect.ValueOf("a")).IsValid())
	am := map[any]string{1: "int", "s": "string", 2.5: "float"}
	av := reflect.ValueOf(am)
	fmt.Println(av.MapIndex(reflect.ValueOf(1)), av.MapIndex(reflect.ValueOf("s")), av.MapIndex(reflect.ValueOf(2.5)), av.MapIndex(reflect.ValueOf(int8(1))).IsValid())
	func() {
		defer func() { fmt.Println("recovered:", recover()) }()
		v.MapIndex(reflect.ValueOf(1))
	}()
	func() {
		defer func() { fmt.Println("recovered:", recover()) }()
		reflect.ValueOf(1).MapKeys()
	}()
}

// Output:
// apple 1 int false false
// banana 2 int false false
// cherry 3 int false false
// false 3
// 1 1 false
// 2 0 true
// 3 1 false
// 1
// 0 false
// int string float false
// recovered: reflect.Value.MapIndex: value of type int is not assignable to type string
// recovered: reflect: call of reflect.Value.MapKeys on int Value
