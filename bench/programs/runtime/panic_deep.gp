// bench: panic_deep
// category: runtime
// size: 2000000
// smoke: 5000
// twin: no
// about: a panic ten calls deep (functions without deferred calls), recovered by a deferred call at the top
package main

import "fmt"

var frames int

//go:noinline
func deep(i, n int) int {
	if n == 0 {
		panic(i)
	}
	r := deep(i, n-1) + 1
	frames++
	return r
}

func risky(i int) (r int) {
	defer func() {
		if v, ok := recover().(int); ok {
			r = v
		}
	}()
	return deep(i, 10)
}

func main() {
	sum := 0
	for i := 0; i < benchN; i++ {
		sum += risky(i)
	}
	fmt.Println(sum, frames)
}
