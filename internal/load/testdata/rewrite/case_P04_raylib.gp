//gocpp:pkg-config raylib
package main

import rl "<raylib.h>" // renamed like a Go import; without a name it is raylib.InitWindow

func main() {
	rl.InitWindow(800, 450, "gocpp + raylib")
	defer rl.CloseWindow()
	rl.SetTargetFPS(60)
	for !rl.WindowShouldClose() {
		rl.BeginDrawing()
		rl.ClearBackground(rl.RAYWHITE) // a macro: CLITERAL(Color){245, 245, 245, 255}
		rl.DrawText("Hello from gocpp!", 190, 200, 20, rl.LIGHTGRAY)
		rl.EndDrawing()
	}
}
