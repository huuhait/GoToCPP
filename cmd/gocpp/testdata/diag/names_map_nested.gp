package main

import "<cstdlib>"

type Point struct{ X, Y int }

func main() {
	m := map[string][]Point{}
	_ = cstdlib.abs(m)
}
