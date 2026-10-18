// phase: 07

// Types of other packages as type arguments: %T and %v of the instances, zero values, equality, and the
// descriptors of unexported types that only the generic code boxes.
package main

import (
	"fmt"

	"example.com/gtypes/gen"
	"example.com/gtypes/shape"
)

type Local struct{ A int }

func main() {
	fmt.Println(gen.Describe(shape.Point{1, 2}), gen.Describe(shape.Square), gen.Describe(&shape.Point{3, 4}))
	fmt.Println(gen.Describe(shape.Make(5)), gen.Describe([]shape.Point{{1, 1}}), gen.Describe(map[shape.Kind]Local{shape.Circle: {1}}))
	fmt.Println(gen.Describe(Local{3}), gen.Describe(shape.Sq{2}), gen.Describe[shape.Shape](shape.Sq{3}), gen.Describe[shape.Shape](nil))
	fmt.Println(gen.Zero[shape.Point](), gen.Zero[shape.Kind](), gen.Zero[*shape.Rect]() == nil, gen.Zero[shape.Shape]() == nil)
	fmt.Println(gen.Same(shape.Point{1, 2}, shape.Point{1, 2}), gen.Same(shape.Square, shape.Circle), gen.Same[shape.Shape](shape.Sq{1}, shape.Sq{1}))
	fmt.Println(gen.TotalArea(shape.Sq{2}, shape.Sq{3}), gen.TotalArea(&shape.Rect{2, 3}), gen.TotalArea[shape.Shape](shape.Sq{1}, &shape.Rect{1, 5}))
	fmt.Println(gen.Index([]shape.Kind{shape.Circle, shape.Square}, shape.Square), gen.Index([]shape.Point{{1, 1}}, shape.Point{2, 2}))
	fmt.Printf("%v %T\n", gen.Box(shape.Make(1)), gen.Box(shape.Make(2)))
	_, ok := gen.Box(shape.Point{}).(shape.Point)
	_, bad := gen.Box(shape.Point{}).(Local)
	fmt.Println(ok, bad)
}


// Output:
// (1,2)|shape.Point square|shape.Kind (3,4)|*shape.Point
// hidden5|shape.hidden [(1,1)]|[]shape.Point map[circle:{1}]|map[shape.Kind]main.Local
// {3}|main.Local {2}|shape.Sq {3}|shape.Sq <nil>|<nil>
// (0,0) circle true true
// true false true
// 13 6 6
// 1 -1
// hidden1 shape.hidden
// true false
