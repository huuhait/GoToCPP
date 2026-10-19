// phase: 09

package main

import (
	"fmt"
	"reflect"
)

func main() {
	zero := 0.0
	nan := zero / zero
	m := map[float64]int{}
	v := reflect.ValueOf(m)
	v.SetMapIndex(reflect.ValueOf(nan), reflect.ValueOf(1))
	v.SetMapIndex(reflect.ValueOf(nan), reflect.ValueOf(2))
	v.SetMapIndex(reflect.ValueOf(1.0), reflect.ValueOf(3))
	fmt.Println(len(m), v.Len())
	fmt.Println(v.MapIndex(reflect.ValueOf(nan)).IsValid(), v.MapIndex(reflect.ValueOf(1.0)))
	sum := 0
	for it := v.MapRange(); it.Next(); {
		sum += int(it.Value().Int())
	}
	fmt.Println(sum)
	v.SetMapIndex(reflect.ValueOf(nan), reflect.Value{})
	fmt.Println(len(m))
	v.Clear()
	fmt.Println(len(m))
}

// Output:
// 3 3
// false 3
// 6
// 3
// 0
