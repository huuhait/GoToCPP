// phase: 03

// The order of a store's panics, as gc's: the key's calls run first, then the value, which
// panics before the entry is taken (so before the nil map's panic); the stdout before the panic
// and the exit code are compared.
package main

import "fmt"

var m map[int]int

func key() int {
	fmt.Println("key")
	return 1
}

func main() {
	ok := map[int]int{}
	ok[key()] = 5
	fmt.Println(ok[1], len(m))
	z := 0
	m[key()] = 7 / z
}
