// phase: 02

// panic with a bool value, four calls deep, exits with code 2 after the
// output printed before it.
package main

import "fmt"

func depth(n int) int {
	fmt.Println("depth", n)
	if n == 3 {
		panic(n == 3)
	}
	return depth(n+1) + 1
}

func main() {
	fmt.Println(depth(0))
}
