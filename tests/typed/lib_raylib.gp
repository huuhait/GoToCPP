// phase: 12
// pkg-config: raylib
// build-only

// raylib, BUILD ONLY (it needs a display): struct literals with the fields out of order (Color is
// {r, g, b, a}), gocpp strings for const char* parameters, and the numbers of a gocpp loop in
// C parameters. InitWindow keeps its title, so it gets a constant.
//
//gocpp:pkg-config raylib
package main

import (
	"fmt"

	rl "<raylib.h>"
)

func main() {
	rl.SetConfigFlags(rl.FLAG_WINDOW_HIDDEN)
	rl.InitWindow(320, 200, "gocpp typed")
	defer rl.CloseWindow()
	rl.SetTargetFPS(60)

	tint := rl.Color{a: 255, b: 40, g: 80, r: 200}
	pos := rl.Vector2{y: 100, x: 50}
	for frame := 0; frame < 3 && !rl.WindowShouldClose(); frame++ {
		label := fmt.Sprint("frame ", frame)
		rl.BeginDrawing()
		rl.ClearBackground(rl.RAYWHITE)
		rl.DrawText(label, 10, 10+frame, 20, tint)
		rl.DrawCircleV(pos, 12, tint)
		rl.EndDrawing()
	}
	rl.SetWindowTitle("typed")
}
