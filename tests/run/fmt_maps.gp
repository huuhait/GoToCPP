// phase: 03

// fmt prints maps with their keys sorted (Go's internal/fmtsort): integers, unsigned integers
// and floats by value (NaN first, -0 equal to +0), strings by their bytes, false before true,
// arrays element by element, structs field by field, named keys by their values; nil and
// empty maps print map[]; maps nested in slices, maps and structs too.
package main

import "fmt"

type key struct {
	a string
	b int
}

type level int

type entry struct {
	name string
	tags map[string]bool
}

func main() {
	zero := 0.0
	inf, nan, negz := 1/zero, zero/zero, -zero
	fmt.Println(map[int]string{3: "c", -1: "a", 2: "b", 0: "z"})
	fmt.Println(map[uint8]int{255: 1, 0: 2, 128: 3})
	fmt.Println(map[float64]int{2.5: 1, -1: 2, inf: 3, -inf: 4, negz: 5})
	fmt.Println(map[float64]int{nan: 1, 1: 2, -inf: 3}, map[float64]int{0: 1, negz: 2})
	fmt.Println(map[string]int{"b": 1, "a": 2, "B": 3, "ab": 4, "": 5, "é": 6})
	fmt.Println(map[bool]string{true: "yes", false: "no"})
	fmt.Println(map[[2]int]bool{{2, 1}: true, {1, 9}: false, {1, 2}: true})
	fmt.Println(map[key]int{{"b", 1}: 1, {"a", 2}: 2, {"a", 1}: 3})
	fmt.Println(map[level]string{3: "high", 1: "low", 2: "mid"})
	var nilM map[string]int
	fmt.Println(nilM, map[string]int{}, len(nilM))
	nested := map[string]map[int][]string{"x": {2: {"b"}, 1: {"a"}}, "a": nil}
	fmt.Println(nested)
	fmt.Println([]map[string]int{{"b": 2, "a": 1}, nil}, entry{"e", map[string]bool{"z": true, "y": false}})
	fmt.Printf("%v|%+v|%d|%s\n", map[string]int{"b": 2, "a": 1}, map[key]int{{"k", 1}: 1}, map[int]int{2: 20, 1: 10}, map[string]string{"k": "v"})
	fmt.Printf("%5v|%-3d|%q\n", map[int]int{1: 2}, map[int]int{1: 2}, map[string]string{"k": "v"})
	m := map[string]int{}
	for i := 0; i < 20; i++ {
		m[string(rune('a'+(i*7)%20))] = i
	}
	fmt.Println(m)
	s := fmt.Sprint(map[int]bool{2: true, 1: false})
	fmt.Println(s, len(s))
}

// Output:
// map[-1:a 0:z 2:b 3:c]
// map[0:2 128:3 255:1]
// map[-Inf:4 -1:2 -0:5 2.5:1 +Inf:3]
// map[NaN:1 -Inf:3 1:2] map[-0:2]
// map[:5 B:3 a:2 ab:4 b:1 é:6]
// map[false:no true:yes]
// map[[1 2]:true [1 9]:false [2 1]:true]
// map[{a 1}:3 {a 2}:2 {b 1}:1]
// map[1:low 2:mid 3:high]
// map[] map[] 0
// map[a:map[] x:map[1:[a] 2:[b]]]
// [map[a:1 b:2] map[]] {e map[y:false z:true]}
// map[a:1 b:2]|map[{a:k b:1}:1]|map[1:10 2:20]|map[k:v]
// map[    1:    2]|map[1  :2  ]|map["k":"v"]
// map[a:0 b:3 c:6 d:9 e:12 f:15 g:18 h:1 i:4 j:7 k:10 l:13 m:16 n:19 o:2 p:5 q:8 r:11 s:14 t:17]
// map[1:false 2:true] 19
