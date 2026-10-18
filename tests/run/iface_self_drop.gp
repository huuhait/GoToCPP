// phase: 05

// A method that drops the last reference to its own interface's box while it runs: the receiver
// stays valid (a value method has its own copy, a pointer method its own pointer) (step 5.4).
package main

import "fmt"

type Shape interface{ Area() int }

var g Shape

type V struct{ a, b int }
type P struct{ a, b int }
type Big struct {
	data [20000]int
	a    int
}

func (v V) Area() int {
	g = nil
	return v.a * v.b
}

func (p *P) Area() int {
	g = nil
	return p.a + p.b
}

func (b Big) Area() int {
	g = nil
	return b.data[19999] + b.a
}

func set(s Shape) Shape { g = s; return s }

func main() {
	fmt.Println(set(V{3, 4}).Area(), g == nil)
	fmt.Println(set(&P{3, 4}).Area(), g == nil)
	g = V{5, 6}
	fmt.Println(g.Area(), g == nil)
	g = &P{5, 6}
	fmt.Println(g.Area(), g == nil)
	b := Big{a: 7}
	b.data[19999] = 8
	g = b
	fmt.Println(g.Area(), g == nil)
	h := struct{ s Shape }{V{2, 2}}
	g = h.s
	fmt.Println(h.s.Area(), g == nil)
	arr := []Shape{V{1, 9}}
	g = arr[0]
	fmt.Println(arr[0].Area(), g == nil)
	var fs []func() Shape
	fs = append(fs, func() Shape { return set(V{4, 4}) })
	fmt.Println(fs[0]().Area())
}

// Output:
// 12 true
// 7 true
// 30 true
// 11 true
// 15 true
// 4 true
// 9 true
// 16
