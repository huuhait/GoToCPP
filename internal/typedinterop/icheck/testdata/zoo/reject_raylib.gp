// Wrong calls of raylib (the dev shell has it).
//
//gocpp:pkg-config raylib
package main

import rl "<raylib.h>"

func main() {
	var c rl.Color
	_ = c
	rl.InitWindow("x", 1, 2)
	rl.DrawText(1)
	_ = c.q
	rl.NoSuchFunction(3)
}
