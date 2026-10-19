package main

import "<cstdlib>"

func main() {
	var c chan<- int
	_ = cstdlib.abs(c)
}
