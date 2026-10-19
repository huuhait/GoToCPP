package main

import "<cstdlib>"

func main() {
	p := &struct{ X, Y int }{}
	_ = cstdlib.abs(p)
}
