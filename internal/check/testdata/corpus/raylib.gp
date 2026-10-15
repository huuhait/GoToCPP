//gocpp:pkg-config raylib

// raylib as a renamed header package with //gocpp:pkg-config (probed with
// the flags that pkg-config gives): macros as values, C structs, vectors.
package main

import rl "<raylib.h>"

type Ball struct {
	pos   rl.Vector2
	speed rl.Vector2
	color rl.Color
}

func (b *Ball) update(dt float32) {
	b.pos.x += b.speed.x * dt
	b.pos.y += b.speed.y * dt
	if b.pos.x < 0 || float64(b.pos.x) > 800 {
		b.speed.x = -b.speed.x
	}
}

func main() {
	rl.InitWindow(800, 450, C.cstr("gocpp + raylib"))
	defer rl.CloseWindow()
	rl.SetTargetFPS(60)
	balls := []Ball{{pos: rl.Vector2{x: 10, y: 20}, speed: rl.Vector2{x: 100, y: 50}, color: rl.RED}}
	for !rl.WindowShouldClose() {
		dt := rl.GetFrameTime()
		for i := range balls {
			balls[i].update(dt)
		}
		rl.BeginDrawing()
		rl.ClearBackground(rl.RAYWHITE)
		for _, b := range balls {
			rl.DrawCircleV(b.pos, 8, b.color)
		}
		rl.DrawText(C.cstr("Hello from gocpp!"), 190, 200, 20, rl.LIGHTGRAY)
		rl.EndDrawing()
	}
}
