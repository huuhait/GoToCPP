// phase: 03

// An index out of range of a package array panics after the calls of its statement ran.
package main

import "fmt"

var ga = [3]int{1, 2, 3}

func f() int {
	fmt.Println("f ran")
	ga[0] = 10
	return 1
}

func main() {
	fmt.Println(ga[0], f())
	i := 3
	fmt.Println(ga[i] + f())
}
