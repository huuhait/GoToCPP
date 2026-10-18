// phase: 05

// A recovered panic is not a failure: the program ends with code 0.
package main

import "fmt"

func main() {
	defer func() {
		fmt.Println("recovered:", recover())
	}()
	var m map[string]int
	m["x"] = 1
	fmt.Println("not reached")
}

// Output:
// recovered: assignment to entry in nil map
