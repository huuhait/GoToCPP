// phase: 03

// Audit case: the fmt shim prints composite values as Go's fmt does (Phase 03 README §2.10):
// nested slices, arrays, maps (keys sorted) and structs, %v %+v %d %s %q on them, pointers to
// composites at the top level (&{...}), String methods on elements, and nil values inside.
package main

import "fmt"

type Point struct{ X, Y int }

type Color int

func (c Color) String() string { return [...]string{"red", "green", "blue"}[c] }

type Shape struct {
	Name   string
	Pts    []Point
	Fill   Color
	Tags   map[string][]int
	Nested struct {
		A [2]bool
		B *Point
	}
}

func main() {
	sh := Shape{Name: "tri", Pts: []Point{{0, 0}, {3, 0}, {0, 4}}, Fill: 2,
		Tags: map[string][]int{"z": {3}, "a": {1, 2}, "m": nil}}
	sh.Nested.A[1] = true
	fmt.Println(sh)
	fmt.Printf("%+v\n", sh)
	fmt.Printf("%v|%d|%s\n", sh.Pts, sh.Pts, []string{"a", "b"})
	fmt.Printf("%q %q\n", []string{"x y", "z"}, map[string]string{"k": "v"})
	cs, cm := []Color{0, 1}, map[Color]Point{1: {1, 1}, 0: {}}
	fmt.Println(&sh.Pts[1], &cs, &cm)
	fmt.Println([][]Color{{0}, {1, 2}, nil}, map[Point]string{{2, 1}: "b", {1, 2}: "a", {1, 1}: "c"})
	fmt.Printf("%5d|%-4v|%6.2f\n", []int{1, 22}, [2]bool{true}, []float64{1.5, 2})
	fmt.Println(fmt.Sprint(Point{1, 2}, "s", 3, 4, "t"), fmt.Sprintln([]int{}, map[int]int{}))
	var np *Point
	var nm map[string]int
	var ns []Point
	fmt.Println(np, nm, ns, len(fmt.Sprint(np)))
}

// Output:
// {tri [{0 0} {3 0} {0 4}] blue map[a:[1 2] m:[] z:[3]] {[false true] <nil>}}
// {Name:tri Pts:[{X:0 Y:0} {X:3 Y:0} {X:0 Y:4}] Fill:blue Tags:map[a:[1 2] m:[] z:[3]] Nested:{A:[false true] B:<nil>}}
// [{0 0} {3 0} {0 4}]|[{0 0} {3 0} {0 4}]|[a b]
// ["x y" "z"] map["k":"v"]
// &{3 0} &[red green] &map[red:{0 0} green:{1 1}]
// [[red] [green blue] []] map[{1 1}:c {1 2}:a {2 1}:b]
// [    1    22]|[true false]|[  1.50   2.00]
// {1 2}s3 4t [] map[]
//
// <nil> map[] [] 5
