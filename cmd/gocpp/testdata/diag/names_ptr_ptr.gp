package main

import "<cstdlib>"

func main() {
	x := 1
	p := &x
	pp := &p
	_ = cstdlib.abs(pp)
}
