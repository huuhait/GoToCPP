// phase: 05

// A failed assertion to a concrete type panics with Go's message (step 5.5).
package main

import "fmt"

func main() {
	var x any = "text"
	fmt.Println("start")
	n := x.(int)
	fmt.Println(n)
}
