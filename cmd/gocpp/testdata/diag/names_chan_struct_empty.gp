package main

import "<cstdlib>"

func main() {
	c := make(chan struct{})
	_ = cstdlib.abs(c)
}
