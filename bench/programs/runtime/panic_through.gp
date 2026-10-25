// bench: panic_through
// category: runtime
// size: 2000000
// smoke: 5000
// twin: no
// about: a panic that two functions with deferred calls let go on before their caller recovers it (gocpp: a throw and two rethrows)
package main

import "fmt"

var deferred int

//go:noinline
func thrower(i int) {
	panic(i)
}

//go:noinline
func inner(i int) {
	defer func() { deferred++ }()
	thrower(i)
}

//go:noinline
func middle(i int) {
	defer func() { deferred += 2 }()
	inner(i)
}

func risky(i int) (r int) {
	defer func() {
		if v, ok := recover().(int); ok {
			r = v
		}
	}()
	middle(i)
	return 0
}

func main() {
	sum := 0
	for i := 0; i < benchN; i++ {
		sum += risky(i)
	}
	fmt.Println(sum, deferred)
}
