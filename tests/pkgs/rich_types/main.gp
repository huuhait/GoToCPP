// phase: 06

// Richer uses of the types of other packages: structs with unexported fields built by methods, defined types over slices, maps and funcs with methods, interfaces that embed interfaces of the same package, a struct that embeds a pointer and an interface of another package, method values and expressions, == and map keys of structs, a defined int type with String in fmt and in switch, and package variables initialized from calls of other packages.

package main

import (
	"fmt"

	"example.com/rich/app"
	"example.com/rich/colors"
	"example.com/rich/geo"
)

type local struct{ geo.Mover }

func main() {
	fmt.Println(geo.Origin, geo.Unit, app.Sum, app.Default.Tf.Twice(geo.Point{0, 0}))
	var s geo.Stack
	s.Push(geo.Point{5, 6})
	s.Push(geo.Point{7, 8})
	fmt.Println(s.Len(), s.Top())
	it := app.Item{Point: geo.Point{2, 3}, Color: colors.Blue, Stack: &s}
	it.Move(1, 1)
	fmt.Println(it.Point, it.X, it.Color, it.Len(), it.Top().Y)
	fmt.Println(colors.Palette, colors.ByColor[colors.Green], len(colors.ByColor))
	fmt.Println(geo.Points{{1, 1}, {2, 2}}.Sum(), geo.ByName{"a": {9, 9}}.Get("a"))
	add := geo.Point.Add
	mv := (*geo.Point).Move
	p := geo.Point{1, 1}
	mv(&p, 2, 2)
	fmt.Println(add(p, p), p == geo.Point{3, 3}, p != geo.Origin)
	var m geo.Mover = &p
	m.Move(1, 0)
	fmt.Println(p)
	var b geo.Both = &geo.Pt3{P: geo.Point{1, 2}, Z: 9}
	fmt.Println(b.Name())
	b.Move(10, 10)
	if pt, ok := b.(*geo.Pt3); ok {
		fmt.Println(pt.P, pt.Z)
	}
	_, isNamed := m.(geo.Named)
	fmt.Println(isNamed)
	l := local{m}
	l.Move(0, 5)
	fmt.Println(p)
	fmt.Println(app.Default.Items[colors.Red][0].X, len(app.Default.Items))
	counts := map[geo.Point]int{{1, 1}: 1}
	counts[geo.Point{1, 1}]++
	fmt.Println(counts)
	switch c := colors.Green; c {
	case colors.Red:
		fmt.Println("red")
	case colors.Green:
		fmt.Println("green!", c)
	}
	fmt.Printf("%v %d %s %+v\n", colors.Blue, colors.Blue, colors.Red, geo.Point{1, 2})
}

// Output:
// <1,2> <3,4> <4,6> <6,8>
// 2 <7,8>
// <3,4> 3 blue 2 8
// [red green blue] g 2
// <3,3> <9,9>
// <6,6> true true
// <4,3>
// pt3-9
// <11,12> 9
// false
// <4,8>
// 1 1
// map[<1,1>:2]
// green! green
// blue 2 red <1,2>
