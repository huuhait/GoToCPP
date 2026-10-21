package main

import "fmt"

func counter() func() int {
	n := 0 // captured and changed: shared between the function and the closure
	return func() int {
		n++
		return n
	}
}

func main() {
	step := 10 // captured by a closure that is called in place: stays on the stack
	add := func(x int) int { return x + step }
	next := counter()
	fmt.Println(add(1), next(), next())

	var fs []func() int
	for i := 0; i < 2; i++ { // each iteration has its own i; the closures keep them
		fs = append(fs, func() int { return i })
	}
	fmt.Println(fs[0](), fs[1]())
}
