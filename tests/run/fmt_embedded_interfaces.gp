// phase: 05

// Interfaces embedded in structs and interface-typed fields: a struct that embeds an interface
// prints through the promoted method set; fields of interface type hold values with methods.
package main

import "fmt"

type Shape interface{ Area() float64 }

type Sq struct{ S float64 }

func (s Sq) Area() float64 { return s.S * s.S }

func (s Sq) String() string { return fmt.Sprintf("Sq(%g)", s.S) }

type Circle struct{ R float64 }

func (c *Circle) Area() float64 { return 3 * c.R * c.R }

type Holder struct {
	Shape
	Name string
}

type NamedShape struct {
	fmt.Stringer
	N int
}

type Both interface {
	Shape
	fmt.Stringer
}

type Gallery struct {
	Items []Shape
	Index map[string]Shape
	Best  Shape
	both  Both
	Any   any
}

func main() {
	h := Holder{Sq{2}, "sq"}
	fmt.Println(h, h.Area())
	fmt.Printf("%v %+v %#v\n", h, h, h)
	ns := NamedShape{Sq{3}, 1}
	fmt.Println(ns)
	fmt.Printf("%v %+v %s\n", ns, ns, ns)
	var empty Holder
	fmt.Printf("%v %+v\n", empty, empty)
	g := Gallery{[]Shape{Sq{1}, &Circle{2}, nil}, map[string]Shape{"a": Sq{4}}, Sq{5}, Sq{6}, Sq{7}}
	s := fmt.Sprintf("%v", g)
	fmt.Println(len(s) > 10, s[:9])
	fmt.Printf("%v\n", Gallery{})
	fmt.Printf("%+v\n", Gallery{Best: Sq{8}})
	var b Both = Sq{9}
	fmt.Println(b, b.Area(), []Both{b})
}

// Output:
// {Sq(2) sq} 4
// {Sq(2) sq} {Shape:Sq(2) Name:sq} main.Holder{Shape:main.Sq{S:2}, Name:"sq"}
// Sq(3)
// Sq(3) Sq(3) Sq(3)
// {<nil> } {Shape:<nil> Name:}
// true {[Sq(1) 0
// {[] map[] <nil> <nil> <nil>}
// {Items:[] Index:map[] Best:Sq(8) both:<nil> Any:<nil>}
// Sq(9) 81 [Sq(9)]
