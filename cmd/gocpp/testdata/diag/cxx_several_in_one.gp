package main

import "<cstdlib>"

type Point struct{ X, Y int }

func main() {
	s := []string{"a"}
	m := map[string][]int{}
	p := &Point{}
	f := func(int) bool { return true }
	c := make(chan []byte)
	_ = cstdlib.abs(s)
	_ = cstdlib.abs(m)
	_ = cstdlib.abs(p)
	_ = cstdlib.abs(f)
	_ = cstdlib.abs(c)
}
