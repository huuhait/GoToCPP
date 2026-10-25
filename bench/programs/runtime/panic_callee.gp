// bench: panic_callee
// category: runtime
// size: 3000000
// smoke: 5000
// twin: no
// about: a panic in a callee (an int or an error), recovered by a deferred call of its caller (gocpp: a C++ throw)
package main

import (
	"errors"
	"fmt"
)

var errBad = errors.New("bad")

//go:noinline
func thrower(i int) {
	if i%2 == 0 {
		panic(i)
	}
	panic(errBad)
}

func risky(i int) (r int) {
	defer func() {
		switch v := recover().(type) {
		case int:
			r = v
		case error:
			r = -1
		}
	}()
	thrower(i)
	return 0
}

func main() {
	sum, bad := 0, 0
	for i := 0; i < benchN; i++ {
		if r := risky(i); r < 0 {
			bad++
		} else {
			sum += r
		}
	}
	fmt.Println(sum, bad)
}
