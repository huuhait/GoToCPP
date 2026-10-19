package main

import "<cstdio>"

type Point struct{ X, Y int }

func main() {
	p := Point{1, 2}
	cstdio.puts(p)
	cstdio.puts(&p)
}
