// phase: 04
// pkg-config: raylib
// display: required

// raylib in opaque mode, BUILD ONLY (the zoo test never runs a program that needs a display):
// InitWindow/WindowShouldClose/BeginDrawing, struct values (rl.Vector2, rl.Color), the color
// macros (rl.RAYWHITE is a compound literal macro), and raymath-free vector math in Go.
//
//gocpp:pkg-config raylib
package main

import (
	"fmt"

	rl "<raylib.h>"
)

type Ball struct{ pos, vel rl.Vector2 }

func (b *Ball) step() {
	b.pos.x += b.vel.x
	b.pos.y += b.vel.y
	if b.pos.x < 0 || b.pos.x > 320 {
		b.vel.x = -b.vel.x
	}
}

func main() {
	rl.SetConfigFlags(rl.FLAG_WINDOW_HIDDEN)
	rl.InitWindow(320, 200, "gocpp zoo")
	defer rl.CloseWindow()
	rl.SetTargetFPS(60)

	ball := Ball{rl.Vector2{x: 100, y: 100}, rl.Vector2{x: 2, y: 1}}
	frames := 0
	for !rl.WindowShouldClose() && frames < 3 {
		ball.step()
		rl.BeginDrawing()
		rl.ClearBackground(rl.RAYWHITE)
		rl.DrawCircleV(ball.pos, 12, rl.MAROON)
		rl.DrawText(rl.TextFormat("frame %d", C.int(frames)), 10, 10, 20, rl.DARKGRAY)
		rl.EndDrawing()
		frames++
	}
	fmt.Println(frames)
}

// Output:
