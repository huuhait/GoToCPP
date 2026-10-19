// Wrong calls of SDL2 (the dev shell has it).
//
//gocpp:pkg-config sdl2
package main

import "<SDL2/SDL.h>"

func main() {
	var r SDL.SDL_Rect
	_ = r
	_ = SDL.SDL_Init("x")
	SDL.SDL_Quit(1)
	_ = r.nosuch
	_ = SDL.SDL_CreateWindow(1, 2)
}
