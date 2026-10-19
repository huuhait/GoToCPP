package main

import "<cstdlib>"

func main() {
	s := []struct{ Name string }{}
	_ = cstdlib.abs(s)
}
