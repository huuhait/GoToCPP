package main

import "<cstdlib>"

type new_ struct{}

func main() {
	var n new_
	_ = cstdlib.abs(n)
}
