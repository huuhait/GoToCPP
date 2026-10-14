// phase: 02

// Modulo by a zero variable of a small unsigned type panics too.
package main

import "fmt"

func main() {
	var a uint16 = 1000
	for d := uint16(3); ; d-- {
		fmt.Println(a % d)
	}
}
