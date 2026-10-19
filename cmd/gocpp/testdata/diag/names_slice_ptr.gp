package main

import "<cstdlib>"

type Point struct{ X, Y int }

func main() {
	s := []*Point{}
	_ = cstdlib.abs(s)
}
