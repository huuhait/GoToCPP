//gocpp:pkg-config raylib
package main

import (
	"./physics.h"   // physics.c in this folder is compiled and linked automatically
	rl "<raylib.h>" // renamed, like a Go import
)

func main() {
	rl.InitWindow(800, 450, "gocpp + raylib")
	defer rl.CloseWindow()

	for !rl.WindowShouldClose() {
		physics.step(rl.GetFrameTime())
		rl.BeginDrawing()
		rl.ClearBackground(rl.RAYWHITE)
		rl.DrawText("Hello from gocpp!", 190, 200, 20, rl.DARKGRAY)
		rl.EndDrawing()
	}
}
