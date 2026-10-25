// bench: panic_repanic
// category: runtime
// size: 2000000
// smoke: 5000
// twin: no
// about: a deferred call that recovers a panic and panics again with a new value, recovered by the caller (a chain of two panics)
package main

import "fmt"

//go:noinline
func thrower(i int) {
	panic(i)
}

//go:noinline
func again(i int) {
	defer func() {
		v := recover().(int)
		panic(v + 1)
	}()
	thrower(i)
}

func risky(i int) (r int) {
	defer func() {
		if v, ok := recover().(int); ok {
			r = v
		}
	}()
	again(i)
	return 0
}

func main() {
	sum := 0
	for i := 0; i < benchN; i++ {
		sum += risky(i)
	}
	fmt.Println(sum)
}
