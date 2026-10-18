// phase: 04

// C++ features at the boundary: overloads (the C++ compiler picks, by the argument types), default
// arguments, reference parameters and returns, enum class and plain enums, operators of a
// user class (+, unary -, +=, ==, <, [], ()), a class template with a static
// member, a function template, and range over a type with begin()/end().
package main

import (
	"fmt"

	"<vector>"

	"./oop.hpp"
)

func main() {
	// Overloads and default arguments.
	fmt.Println(int(oop.scale(4)), float64(oop.scale(4.0)), string(oop.scale(std.string("ab"))))
	fmt.Println(int(oop.volume(2)), int(oop.volume(2, 5)), int(oop.volume(2, 5, 7)))

	// Reference parameters: the C++ function writes through them.
	var q, r C.int
	oop.divmod(17, 5, q, r)
	fmt.Println(int(q), int(r))
	a, b := C.int(1), C.int(2)
	a = oop.pick(a, b, false) // a reference result is read as a value (a call is never a place)
	fmt.Println(int(a), int(b), int(oop.pick(a, b, true)))
	fmt.Println(int(oop.sum_all(std.vector[C.int]{1, 2, 3})))

	// Enums: an enum class value needs `::`, a plain enum's constants are namespace members.
	col := oop.Color.Green
	fmt.Println(C.GoString(oop.color_name(col)), C.GoString(oop.color_name(oop.Color.Blue)), int(col), int(oop.Color.Blue))
	fmt.Println(int(oop.Low), int(oop.Mid), int(oop.High), col == oop.Color.Green)

	// Operators of a class.
	u, v := oop.Vec2(1, 2), oop.Vec2(10, 20)
	w := u + v
	w += oop.Vec2(100, 100)
	n := -w
	fmt.Println(int(w.x), int(w.y), int(n.x), w == oop.Vec2(111, 122), u < v, v < u)
	fmt.Println(int(w[0]), int(w[1]), int(w(2)))
	var zero oop.Vec2
	fmt.Println(int(zero.x), int(zero.y), zero == oop.Vec2())

	// A class template: its static member, member calls, and C++ iteration.
	var st oop.Stack[int]
	st.push(5)
	st.push(6)
	st.push(7)
	fmt.Println(int(oop.Stack[int].kind), int(st.size()), int(st.pop()), int(st.size()))
	for i, x := range st {
		fmt.Println(int(i), int(x))
	}
	names := oop.Stack[std.string]{}
	names.push("x")
	fmt.Println(string(names.pop()), int(oop.twice[int](21)), string(oop.twice(std.string("go"))))

	sum := 0
	for _, x := range (oop.Ring{}) {
		sum += int(x)
	}
	fmt.Println(sum)
}

// Output:
// 8 6 abab
// 12 30 70
// 3 2
// 2 2 2
// 6
// green blue 5 6
// 1 2 10 true
// 111 122 -111 true true false
// 111 122 466
// 0 0 true
// 7 3 7 2
// 0 5
// 1 6
// x 42 gogo
// 100
