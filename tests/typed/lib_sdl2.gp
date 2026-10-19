// phase: 12
// pkg-config: sdl2
// build-only

// SDL2, BUILD ONLY (it needs a display): SDL_Rect keyed out of order, gocpp strings for the title
// and the hint, SDL_SetWindowTitle with a variable.
//
//gocpp:pkg-config sdl2
package main

import (
	"fmt"

	"<SDL2/SDL.h>"
)

func main() {
	if SDL.SDL_Init(SDL.SDL_INIT_VIDEO) != 0 {
		fmt.Println("init failed:", C.GoString(SDL.SDL_GetError()))
		return
	}
	defer SDL.SDL_Quit()
	hint := "1"
	SDL.SDL_SetHint(SDL.SDL_HINT_RENDER_VSYNC, hint)
	title := "typed " + fmt.Sprint(1)
	win := SDL.SDL_CreateWindow(title, SDL.SDL_WINDOWPOS_CENTERED, SDL.SDL_WINDOWPOS_CENTERED, 320, 200, SDL.SDL_WINDOW_HIDDEN)
	if win == nil {
		return
	}
	defer SDL.SDL_DestroyWindow(win)
	SDL.SDL_SetWindowTitle(win, title+"!")
	ren := SDL.SDL_CreateRenderer(win, -1, SDL.SDL_RENDERER_SOFTWARE)
	defer SDL.SDL_DestroyRenderer(ren)

	for i := 0; i < 3; i++ {
		box := SDL.SDL_Rect{h: 50, w: 100, y: 20 + i, x: 10}
		SDL.SDL_SetRenderDrawColor(ren, 200, 80, 40, 255)
		SDL.SDL_RenderFillRect(ren, &box)
		SDL.SDL_RenderPresent(ren)
	}
}
