package main

import "fmt"

type Big [64]int

func Show[T any](x T) {
	var y T = x
	f := func() T { return y }
	fmt.Println(any(f()), len(fmt.Sprint(x)))
}

func main() {
	Show(1)
	Show(Big{})
	Show("s")
}
