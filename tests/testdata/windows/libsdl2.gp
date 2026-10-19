// phase: 14

// SDL2 built for MinGW-w64 (step 14.4, ADR-082), BUILD ONLY: the program is linked against the import library and
// not run (it needs the SDL2 DLL and a display). The zoo program of tests/zoo/sdl2.gp on Windows.
//
// SDL_Init/SDL_CreateWindow/SDL_CreateRenderer, an SDL_Event union read by type, SDL_Rect
// literals, Uint32 flags, and SDL_main (SDL.h defines `main` as a macro: gocpp protects its
// own `main`).
//
//gocpp:ldflags -lmingw32 -lSDL2main -l:libSDL2.dll.a
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

	win := SDL.SDL_CreateWindow("gocpp zoo", SDL.SDL_WINDOWPOS_CENTERED, SDL.SDL_WINDOWPOS_CENTERED, 320, 200, SDL.SDL_WINDOW_HIDDEN)
	if win == nil {
		fmt.Println("window failed")
		return
	}
	defer SDL.SDL_DestroyWindow(win)
	ren := SDL.SDL_CreateRenderer(win, -1, SDL.SDL_RENDERER_SOFTWARE)
	defer SDL.SDL_DestroyRenderer(ren)

	box := SDL.SDL_Rect{10, 20, 100, 50}
	for frame := 0; frame < 3; frame++ {
		var ev SDL.SDL_Event
		for SDL.SDL_PollEvent(&ev) != 0 {
			if ev.type == SDL.SDL_QUIT {
				return
			}
		}
		SDL.SDL_SetRenderDrawColor(ren, 0, 0, 0, 255)
		SDL.SDL_RenderClear(ren)
		SDL.SDL_SetRenderDrawColor(ren, 200, 80, 40, 255)
		box.x += 5
		SDL.SDL_RenderFillRect(ren, &box)
		SDL.SDL_RenderPresent(ren)
		SDL.SDL_Delay(1)
	}
}
