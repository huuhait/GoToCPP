// Opaque values in every kind of statement and expression: if and switch
// with init statements, type switches, labeled loops, channels, select,
// interfaces, builtins, arrays, maps and struct values of Go types with C
// parts.
package main

import (
	"fmt"
	"strings"

	"./geom.h"
	"<cstdio>"
	"<map>"
	"<string>"
	"<vector>"
)

type Shape interface{ Area() float64 }

type Sprite struct {
	pos   geom.point
	tags  []string
	cells [4]geom.point
	byKey map[string]geom.point
}

type Layer struct {
	Sprite // embedded Go struct with C fields
	depth  int
}

func (s Sprite) Area() float64 { return float64(s.pos.x * s.pos.y) }

func pick(ok bool) geom.point {
	if ok {
		return geom.make_point(1, 1)
	}
	return geom.make_point(0, 0)
}

func main() {
	v := std.vector[int]{1, 2, 3}
	if n := int(v.size()); n > 2 {
		fmt.Println("big", n)
	}
	switch x := v.front(); x {
	case 1, 2:
		fmt.Println("small")
	default:
		fmt.Println(x)
	}
	var any1 any = v.back()
	switch t := any1.(type) {
	case int:
		fmt.Println(t + 1)
	case fmt.Stringer:
		fmt.Println(t.String())
	}
	f := cstdio.fopen(C.cstr("x"), C.cstr("r"))
	if f != nil {
		defer cstdio.fclose(f)
	}
outer:
	for i, n := range v {
		for j := 0; j < 3; j++ {
			if int(n) == j {
				continue outer
			}
			if i > 5 {
				break outer
			}
		}
	}
	ch := make(chan geom.point, 1)
	ch <- geom.make_point(2, 3)
	select {
	case p := <-ch:
		fmt.Println(p.x)
	default:
	}
	var sh Shape = Sprite{pos: pick(true)}
	fmt.Println(sh.Area())
	l := Layer{Sprite: Sprite{tags: []string{"a"}}, depth: 1}
	l.pos = geom.make_point(4, 5)
	l.cells[2] = l.pos
	l.byKey = map[string]geom.point{"o": l.pos}
	fmt.Println(l.pos.x, l.cells[2].y, len(l.cells), len(l.byKey), l.depth, strings.Join(l.tags, ","))
	names := std.map[std.string, std.vector[int]]{}
	names["a"].push_back(1)
	names.erase(std.string("a"))
	fmt.Println(len(l.tags), cap(l.tags), names.size(), v.data() != nil)
	var pts []geom.point
	pts = append(pts, pick(false), l.pos)
	copy(pts, pts[1:])
	fmt.Println(len(pts), pts[0].x)
	arr := [3]geom.point{}
	arr[0].x = 2
	for i := range arr {
		arr[i].y = float64(i)
	}
	m := map[int]geom.point{}
	m[1] = arr[0]
	delete(m, 1)
	q, ok := m[2]
	fmt.Println(q.x, ok)
	func() {
		defer fmt.Println("deferred", int(v.size()))
	}()
	s := fmt.Sprint(v.front(), geom.RED)
	fmt.Println(s, strings.Repeat("x", int(v.size())))
}
