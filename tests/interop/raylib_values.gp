// phase: 04
// pkg-config: raylib

// Literals of raylib's C structs (step 4.6), with raylib functions that need no window: a
// positional Vector2 and Color, keyed Rectangles, and a macro that is a C compound literal
// (MAROON is CLITERAL(Color){190, 33, 55, 255}).
//
//gocpp:pkg-config raylib
package main

import (
	"fmt"

	rl "<raylib.h>"
)

func main() {
	a := rl.Rectangle{0, 0, 10, 10}
	b := rl.Rectangle{x: 5, y: 5, width: 10, height: 10}
	c := rl.Rectangle{x: 20, width: 1, height: 1}
	fmt.Println(bool(rl.CheckCollisionRecs(a, b)), bool(rl.CheckCollisionRecs(a, c)))
	hit := rl.GetCollisionRec(a, b)
	fmt.Println(float64(hit.x), float64(hit.width))
	fmt.Println(bool(rl.CheckCollisionPointRec(rl.Vector2{3, 4}, a)), bool(rl.CheckCollisionPointRec(rl.Vector2{y: 30}, a)))
	red := rl.Color{255, 0, 0, 255}
	fmt.Println(uint32(rl.ColorToInt(red)), uint32(rl.ColorToInt(rl.Color{a: 255})))
	m := rl.MAROON
	half := rl.ColorAlpha(m, 0.5)
	fmt.Println(int(m.r), int(half.a))
}

// Output:
// true false
// 5 5
// true false
// 4278190335 255
// 190 127
