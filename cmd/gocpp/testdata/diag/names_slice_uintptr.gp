package main

import "<cstdlib>"

func main() {
	a := []uintptr{1}
	_ = cstdlib.abs(a)
}
