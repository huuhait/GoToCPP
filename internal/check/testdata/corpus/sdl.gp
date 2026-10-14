// pkg-config: sdl2
//
// SDL2 as a header package, found without //gocpp:pkg-config (the
// namespace probe checks the header): pointers to C structs, flags,
// events and a struct literal of a C type.
package main

import (
	"fmt"

	"<SDL2/SDL.h>"
)

type Game struct {
	win      *SDL.SDL_Window
	renderer *SDL.SDL_Renderer
	running  bool
	frames   int
}

func (g *Game) handle(e *SDL.SDL_Event) {
	if e.type == SDL.SDL_QUIT {
		g.running = false
	}
}

func (g *Game) draw() {
	SDL.SDL_SetRenderDrawColor(g.renderer, 0, 0, 0, 255)
	SDL.SDL_RenderClear(g.renderer)
	r := SDL.SDL_Rect{x: 10, y: 10, w: 100, h: 50}
	r.x += 5
	SDL.SDL_RenderFillRect(g.renderer, &r)
	SDL.SDL_RenderPresent(g.renderer)
	g.frames++
}

func main() {
	if SDL.SDL_Init(SDL.SDL_INIT_VIDEO|SDL.SDL_INIT_TIMER) != 0 {
		fmt.Println(C.GoString(SDL.SDL_GetError()))
		return
	}
	defer SDL.SDL_Quit()
	g := &Game{running: true}
	g.win = SDL.SDL_CreateWindow(C.cstr("corpus"), SDL.SDL_WINDOWPOS_CENTERED, SDL.SDL_WINDOWPOS_CENTERED, 640, 480, 0)
	g.renderer = SDL.SDL_CreateRenderer(g.win, -1, SDL.SDL_RENDERER_ACCELERATED)
	var e SDL.SDL_Event
	for g.running && g.frames < 3 {
		for SDL.SDL_PollEvent(&e) != 0 {
			g.handle(&e)
		}
		g.draw()
		SDL.SDL_Delay(16)
	}
	ticks := uint32(SDL.SDL_GetTicks())
	fmt.Println(g.frames, ticks > 0)
	SDL.SDL_DestroyWindow(g.win)
}
