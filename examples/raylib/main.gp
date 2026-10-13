// phase: 04
// pkg-config: raylib
// display: required

// Raylib opens a window and runs a game loop with the raylib C library: a
// ball bounces inside the window until it is closed (Esc or the close
// button). Space pauses the ball.
//
// When the environment variable GOCPP_SMOKE is set, the window is hidden
// and closes after a few frames, so the program can run in CI. (A Phase 04
// program can't read its arguments yet: os.Args comes with Phase 09.)
//
//gocpp:pkg-config raylib
package main

import (
	"fmt"

	"<cstdlib>"
	rl "<raylib.h>"
)

const (
	width       = 800
	height      = 450
	smokeFrames = 3
)

// Ball moves a fixed step every frame, so a run is the same on every
// machine whatever its frame rate.
type Ball struct {
	X, Y, DX, DY, R float32
}

// Step moves the ball one frame and bounces it off the edges.
func (b *Ball) Step() {
	b.X += b.DX
	b.Y += b.DY
	if b.X < b.R || b.X > width-b.R {
		b.DX = -b.DX
	}
	if b.Y < b.R || b.Y > height-b.R {
		b.DY = -b.DY
	}
}

// run shows the window until it is closed, or for smokeFrames frames in
// smoke mode, and returns the number of frames it drew.
func run(ball *Ball, smoke bool) int {
	rl.SetTraceLogLevel(rl.LOG_ERROR) // raylib logs to stdout; keep it for our own lines
	if smoke {
		rl.SetConfigFlags(rl.FLAG_WINDOW_HIDDEN)
	}
	rl.InitWindow(width, height, "gocpp + raylib")
	if !rl.IsWindowReady() {
		fmt.Println("raylib: no window (is there a display?)")
		return 0
	}
	defer rl.CloseWindow()
	fmt.Println("window:", int(rl.GetScreenWidth()), "x", int(rl.GetScreenHeight()))
	rl.SetTargetFPS(60)

	frames := 0
	paused := false
	for !rl.WindowShouldClose() && !(smoke && frames == smokeFrames) {
		if rl.IsKeyPressed(rl.KEY_SPACE) {
			paused = !paused
		}
		if !paused {
			ball.Step()
		}
		rl.BeginDrawing()
		rl.ClearBackground(rl.RAYWHITE) // a macro: CLITERAL(Color){245, 245, 245, 255}
		rl.DrawCircleV(rl.Vector2{ball.X, ball.Y}, ball.R, rl.MAROON)
		rl.DrawText(C.cstr(fmt.Sprintf("frame %d", frames)), 10, 10, 20, rl.DARKGRAY)
		rl.DrawText("Space: pause, Esc: quit", 10, 40, 20, rl.LIGHTGRAY)
		rl.EndDrawing()
		frames++
	}
	return frames
}

func main() {
	smoke := std.getenv("GOCPP_SMOKE") != nil
	ball := &Ball{X: width / 2, Y: height / 2, DX: 4, DY: 3, R: 20}
	frames := run(ball, smoke)
	fmt.Println("frames:", frames, "ball at:", int(ball.X), int(ball.Y))
	fmt.Println("window closed")
}
