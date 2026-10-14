// main.gp
package main

import (
	"fmt"
	"strings"

	"./physics.h" // your own C header: package physics (physics.c here is compiled too)
	"<cmath>"     // C++ headers make their namespace usable: std.sqrt
	"<vector>"
)

type Vec2 struct {
	X, Y float64
}

func (v Vec2) Len() float64 {
	return std.sqrt(v.X*v.X + v.Y*v.Y)
}

func (v *Vec2) Scale(k float64) {
	v.X *= k
	v.Y *= k
}

func main() {
	v := &Vec2{X: 3, Y: 4}
	v.Scale(2)
	fmt.Println("len:", v.Len())

	nums := std.vector[int]{1, 2, 3} // a real std::vector<int64_t>
	nums.push_back(4)
	for i, n := range nums {
		fmt.Println(i, n)
	}

	physics.step(0.016) // a plain C function from physics.h
	fmt.Println(strings.ToUpper("done"))
}
