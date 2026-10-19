package main

import "<cstdlib>"

type Point struct{ X, Y int }

func main() {
	c := make(chan Point, 1)
	_ = cstdlib.abs(c)
}
