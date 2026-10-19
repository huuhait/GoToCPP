package main

import "<cstdlib>"

func main() {
	c := make(chan int)
	_ = cstdlib.abs(c)
}
