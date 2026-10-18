// phase: 05

// The read-only rules of reflect for embedded fields: emb is unexported and embedded in Pub, Pub
// is exported and embedded in the unexported hid, Top embeds hid and *pemb; exported fields of an
// unexported embedded struct are open again, everything inside an unexported field is closed; the
// same inside slices, maps, arrays, pointers and interfaces.
package main

import "fmt"

type Celsius float64

func (c Celsius) String() string { return fmt.Sprintf("%.1f°C", float64(c)) }

type Pub2 struct{ D Celsius }

type emb struct {
	C Celsius
	c Celsius
	L []Celsius
	M map[string]Celsius
	P Pub2
	p Pub2
}

type Pub struct {
	emb
	E Celsius
}

type hid struct {
	Pub
	F Celsius
	g Celsius
}

type pemb struct{ H Celsius }

type Top struct {
	hid
	*pemb
	Pubs []Pub
	pubs []Pub
	x    Pub
}

func main() {
	e := emb{1, 2, []Celsius{3}, map[string]Celsius{"k": 4}, Pub2{5}, Pub2{6}}
	pub := Pub{e, 7}
	fmt.Println(pub)
	fmt.Printf("%+v\n%v\n%s\n%d\n", pub, &pub, []Pub{pub}, pub)
	fmt.Println(map[string]Pub{"m": pub}, [1]Pub{pub})
	h := hid{pub, 8, 9}
	fmt.Println(h)
	fmt.Printf("%+v\n", h)
	t := Top{h, nil, []Pub{pub}, []Pub{pub}, pub}
	fmt.Println(t)
	fmt.Printf("%+v\n", &t)
	fmt.Println([]Top{t}, map[int]Top{1: t})
	fmt.Println(e, struct{ emb }{e}, struct{ x emb }{e})
	var a any = pub
	fmt.Println(a, []any{pub, e}, struct{ A any; a any }{pub, pub})
}

// Output:
// {{1.0°C 2 [3.0°C] map[k:4.0°C] {5.0°C} {6}} 7.0°C}
// {emb:{C:1.0°C c:2 L:[3.0°C] M:map[k:4.0°C] P:{D:5.0°C} p:{D:6}} E:7.0°C}
// &{{1.0°C 2 [3.0°C] map[k:4.0°C] {5.0°C} {6}} 7.0°C}
// [{{1.0°C %!s(main.Celsius=2) [3.0°C] map[k:4.0°C] {5.0°C} {%!s(main.Celsius=6)}} 7.0°C}]
// {{%!d(main.Celsius=1) %!d(main.Celsius=2) [%!d(main.Celsius=3)] map[%!d(string=k):%!d(main.Celsius=4)] {%!d(main.Celsius=5)} {%!d(main.Celsius=6)}} %!d(main.Celsius=7)}
// map[m:{{1.0°C 2 [3.0°C] map[k:4.0°C] {5.0°C} {6}} 7.0°C}] [{{1.0°C 2 [3.0°C] map[k:4.0°C] {5.0°C} {6}} 7.0°C}]
// {{{1.0°C 2 [3.0°C] map[k:4.0°C] {5.0°C} {6}} 7.0°C} 8.0°C 9}
// {Pub:{emb:{C:1.0°C c:2 L:[3.0°C] M:map[k:4.0°C] P:{D:5.0°C} p:{D:6}} E:7.0°C} F:8.0°C g:9}
// {{{{1.0°C 2 [3.0°C] map[k:4.0°C] {5.0°C} {6}} 7.0°C} 8.0°C 9} <nil> [{{1.0°C 2 [3.0°C] map[k:4.0°C] {5.0°C} {6}} 7.0°C}] [{{1 2 [3] map[k:4] {5} {6}} 7}] {{1 2 [3] map[k:4] {5} {6}} 7}}
// &{hid:{Pub:{emb:{C:1.0°C c:2 L:[3.0°C] M:map[k:4.0°C] P:{D:5.0°C} p:{D:6}} E:7.0°C} F:8.0°C g:9} pemb:<nil> Pubs:[{emb:{C:1.0°C c:2 L:[3.0°C] M:map[k:4.0°C] P:{D:5.0°C} p:{D:6}} E:7.0°C}] pubs:[{emb:{C:1 c:2 L:[3] M:map[k:4] P:{D:5} p:{D:6}} E:7}] x:{emb:{C:1 c:2 L:[3] M:map[k:4] P:{D:5} p:{D:6}} E:7}}
// [{{{{1.0°C 2 [3.0°C] map[k:4.0°C] {5.0°C} {6}} 7.0°C} 8.0°C 9} <nil> [{{1.0°C 2 [3.0°C] map[k:4.0°C] {5.0°C} {6}} 7.0°C}] [{{1 2 [3] map[k:4] {5} {6}} 7}] {{1 2 [3] map[k:4] {5} {6}} 7}}] map[1:{{{{1.0°C 2 [3.0°C] map[k:4.0°C] {5.0°C} {6}} 7.0°C} 8.0°C 9} <nil> [{{1.0°C 2 [3.0°C] map[k:4.0°C] {5.0°C} {6}} 7.0°C}] [{{1 2 [3] map[k:4] {5} {6}} 7}] {{1 2 [3] map[k:4] {5} {6}} 7}}]
// {1.0°C 2 [3.0°C] map[k:4.0°C] {5.0°C} {6}} {{1.0°C 2 [3.0°C] map[k:4.0°C] {5.0°C} {6}}} {{1 2 [3] map[k:4] {5} {6}}}
// {{1.0°C 2 [3.0°C] map[k:4.0°C] {5.0°C} {6}} 7.0°C} [{{1.0°C 2 [3.0°C] map[k:4.0°C] {5.0°C} {6}} 7.0°C} {1.0°C 2 [3.0°C] map[k:4.0°C] {5.0°C} {6}}] {{{1.0°C 2 [3.0°C] map[k:4.0°C] {5.0°C} {6}} 7.0°C} {{1 2 [3] map[k:4] {5} {6}} 7}}
