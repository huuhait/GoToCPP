// bench: panic_runtime_error
// category: runtime
// size: 3000000
// smoke: 5000
// twin: no
// about: an index out of range in a callee, recovered by a deferred call of its caller (a runtime.Error)
package main

import "fmt"

//go:noinline
func at(s []int, i int) int {
	return s[i]
}

func get(s []int, i int) (v int, ok bool) {
	defer func() {
		if _, isErr := recover().(error); isErr {
			ok = false
		}
	}()
	return at(s, i), true
}

func main() {
	s := []int{1, 2, 3}
	sum, bad := 0, 0
	for i := 0; i < benchN; i++ {
		if v, ok := get(s, i%5); ok {
			sum += v
		} else {
			bad++
		}
	}
	fmt.Println(sum, bad)
}
