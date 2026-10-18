// phase: 05

// Structs: %v {1 2}, %+v with field names, %#v in Go syntax, %T; every kind of field; nested and
// empty structs.
package main

import "fmt"

type Point struct{ X, Y int }

type Rec struct {
	Name   string
	Age    int
	Score  float64
	OK     bool
	R      rune
	B      byte
	Tags   []string
	Attrs  map[string]int
	Pt     Point
	Inner  struct{ A, B int }
	Arr    [2]bool
	Nil    []int
	NilMap map[string]int
	Any    any
}

type Empty struct{}

type Blank struct {
	A int
	_ int
	B string
}

func main() {
	r := Rec{"ann", 31, 9.5, true, 'x', 7, []string{"a", "b"}, map[string]int{"z": 1, "a": 2}, Point{1, 2}, struct{ A, B int }{3, 4}, [2]bool{true, false}, nil, nil, 42}
	fmt.Printf("%v\n%+v\n%#v\n%T\n", r, r, r, r)
	fmt.Printf("%v %+v %#v %T\n", Empty{}, Empty{}, Empty{}, Empty{})
	fmt.Printf("%v %+v %#v\n", Blank{A: 1, B: "x"}, Blank{A: 1, B: "x"}, Blank{A: 1, B: "x"})
	fmt.Printf("%v %+v %#v\n", Point{}, &Point{3, 4}, &Point{5, 6})
	fmt.Printf("%v %+v %#v\n", struct{}{}, struct{ A int }{1}, struct {
		A int `json:"a"`
		b string
	}{1, "x"})
	fmt.Println(r, Point{7, 8}, &Point{9, 10})
}

// Output:
// {ann 31 9.5 true 120 7 [a b] map[a:2 z:1] {1 2} {3 4} [true false] [] map[] 42}
// {Name:ann Age:31 Score:9.5 OK:true R:120 B:7 Tags:[a b] Attrs:map[a:2 z:1] Pt:{X:1 Y:2} Inner:{A:3 B:4} Arr:[true false] Nil:[] NilMap:map[] Any:42}
// main.Rec{Name:"ann", Age:31, Score:9.5, OK:true, R:120, B:0x7, Tags:[]string{"a", "b"}, Attrs:map[string]int{"a":2, "z":1}, Pt:main.Point{X:1, Y:2}, Inner:struct { A int; B int }{A:3, B:4}, Arr:[2]bool{true, false}, Nil:[]int(nil), NilMap:map[string]int(nil), Any:42}
// main.Rec
// {} {} main.Empty{} main.Empty
// {1 0 x} {A:1 _:0 B:x} main.Blank{A:1, _:0, B:"x"}
// {0 0} &{X:3 Y:4} &main.Point{X:5, Y:6}
// {} {A:1} struct { A int "json:\"a\""; b string }{A:1, b:"x"}
// {ann 31 9.5 true 120 7 [a b] map[a:2 z:1] {1 2} {3 4} [true false] [] map[] 42} {7 8} &{9 10}
