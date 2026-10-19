package main

import "<cstdlib>"

func main() {
	var v interface{ M() }
	_ = cstdlib.abs(v)
}
