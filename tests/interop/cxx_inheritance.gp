// phase: 04

// Classes with virtual functions: a call through a base pointer or reference reaches the
// override, a derived object's members and base members are both reachable, a static member is
// read with `::`, and objects held by unique_ptr/shared_ptr are called through `(*p).m()`
// (C++ smart pointers are not auto-dereferenced).
package main

import (
	"fmt"

	"<memory>"
	"<vector>"

	"./oop.hpp"
)

func main() {
	r := oop.Rect(2, 3)
	sq := oop.Square(4)
	c := oop.Circle(2)
	fmt.Println(int(oop.Shape.live))

	// Through the exact type, through a pointer and through a base reference.
	fmt.Println(float64(r.area()), r.name(), float64(sq.area()), sq.name(), float64(c.area()), c.name())
	p := &sq
	fmt.Println(p.name(), float64(p.w), p.describe())
	fmt.Println(float64(oop.area_of(r)), float64(oop.area_of(sq)), oop.name_of(&sq), oop.name_of(&c))

	// The base pointer's static type is Shape, the call is virtual.
	var base *oop.Shape = &sq
	fmt.Println(base.name(), float64(base.area()), base.describe())

	// unique_ptr<Shape> from a factory: a different dynamic type each time.
	for kind := 0; kind < 3; kind++ {
		s := oop.make_shape(C.int(kind))
		fmt.Println(kind, (*s).describe(), int(oop.Shape.live))
	}
	fmt.Println(int(oop.Shape.live))

	// shared_ptr in a container, shared with a copy.
	shapes := std.vector[std.shared_ptr[oop.Shape]]{}
	shapes.push_back(std.make_shared[oop.Rect](1, 1))
	shapes.push_back(std.make_shared[oop.Square](3))
	extra := shapes[1]
	fmt.Println(float64(oop.total_area(shapes)), int(extra.use_count()), int(oop.Shape.live))
	for i, s := range shapes {
		fmt.Println(int(i), (*s).name())
	}
}

// Output:
// 3
// 6 rect 16 square 12 shape
// square 4 square:16
// 6 16 square shape
// square 16 square:16
// 0 rect:6 4
// 1 square:16 4
// 2 shape:12 4
// 3
// 10 2 5
// 0 rect
// 1 square
