package main

import "<cstdlib>"

func main() {
	a, b := []int{1}, []int64{1}
	_ = cstdlib.abs(a) + cstdlib.abs(b)
}
