package main

import "<cstdlib>"

func main() {
	f := func(a int) int { return a }
	_ = cstdlib.abs(f)
}
