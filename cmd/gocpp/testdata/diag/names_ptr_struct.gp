package main

import "<cstdlib>"

type Point struct{ X, Y int }

func main() {
	p := &Point{1, 2}
	_ = cstdlib.abs(p)
}
