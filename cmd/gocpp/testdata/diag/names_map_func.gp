package main

import "<cstdlib>"

func main() {
	m := map[int]func(string) bool{}
	_ = cstdlib.abs(m)
}
