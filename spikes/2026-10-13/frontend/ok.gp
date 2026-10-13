package main

import (
	"fmt"
	"strings"

	"<vector>"
	"<cmath>"
	"<cstdio>"
	"<filesystem>"
	"<SDL2/SDL.h>"
	"./native/physics.h"
	cstring "<string.h>"
)

type Vec2 struct{ X, Y float64 }

func (v Vec2) Len() float64 { return std.sqrt(v.X*v.X + v.Y*v.Y) }

func (v *Vec2) Scale(k float64) { v.X *= k; v.Y *= k }

func render(win *SDL.SDL_Window) { SDL.SDL_ShowWindow(win) }

func main() {
	v := std.vector[int]{1, 2, 3}
	v.push_back(4)
	for i, n := range v {
		fmt.Println(i, n)
	}
	a, b := std.minmax(1, 2)
	fmt.Println(a + b)

	var f *cstdio.FILE = cstdio.fopen("x.txt", "r")
	defer cstdio.fclose(f)

	arr := std.vector[int]{}
	var total int = arr.size()
	if v.empty() {
		total++
	}
	p := std.filesystem.path("/tmp")
	fmt.Println(p.string(), total, Vec2{3, 4}.Len(), strings.ToUpper("ok"))

	physics.step(0.016)
	flags := SDL.SDL_INIT_VIDEO | SDL.SDL_INIT_AUDIO
	SDL.SDL_Init(flags)
	render(SDL.SDL_CreateWindow("demo", 0, 0, 640, 480, 0))

	s := "hello"
	n := cstring.strlen(C.cstr(s))
	w := &Vec2{1, 2}
	w.Scale(2)
	w.X = physics.get_x()
	m := map[string]C.int{"a": 1}
	{
		std := 3 // a local named std must hide the namespace
		_ = std + 1
	}
	fmt.Println(m, n, std.string("x").size(), cstdio.EOF)
}
