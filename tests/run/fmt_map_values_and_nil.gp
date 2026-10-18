// phase: 05

// Maps: nil and empty, maps of maps, slices, structs and pointers, %#v with the type name, maps
// inside structs and interfaces.
package main

import "fmt"

type S struct{ M map[string]int }

func main() {
	var nm map[string]int
	em := map[string]int{}
	fmt.Println(nm, em, nm == nil, em == nil, len(nm))
	fmt.Printf("%v %+v %#v %#v %T\n", nm, nm, nm, em, nm)
	fmt.Printf("%v %#v\n", S{}, S{})
	fmt.Printf("%v %#v\n", S{map[string]int{"a": 1}}, S{map[string]int{"a": 1}})
	var a any = map[string]int{"q": 1}
	fmt.Printf("%v %#v %T\n", a, a, a)
	fmt.Printf("%v\n", []map[string]int{{"a": 1}, nil, {}})
	fmt.Printf("%v\n", map[string][]string{"b": {"x", "y"}, "a": nil})
	fmt.Printf("%v\n", map[int]map[int]int{2: {1: 1}, 1: {2: 2}})
	fmt.Printf("%v %v\n", map[string]any{"z": 1, "a": "s", "m": nil, "k": []int{1}}, map[string]error{"e": nil})
	fmt.Printf("%v %v\n", map[int]struct{}{2: {}, 1: {}}, map[string]bool{"t": true, "f": false})
}

// Output:
// map[] map[] true false 0
// map[] map[] map[string]int(nil) map[string]int{} map[string]int
// {map[]} main.S{M:map[string]int(nil)}
// {map[a:1]} main.S{M:map[string]int{"a":1}}
// map[q:1] map[string]int{"q":1} map[string]int
// [map[a:1] map[] map[]]
// map[a:[] b:[x y]]
// map[1:map[2:2] 2:map[1:1]]
// map[a:s k:[1] m:<nil> z:1] map[e:<nil>]
// map[1:{} 2:{}] map[f:false t:true]
