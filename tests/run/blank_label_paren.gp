// phase: 08

// The blank label and a parenthesized blank on the left of an assignment
// (Go issues 52278, 70175 and bug420; our own program).
package main

import "fmt"

func labels() {
_:
	fmt.Println("after the first blank label")
_:
}

func main() {
	labels()
	(_) = 0
	var x int
	(x) = 5
	(_), x = 1, x+1
	fmt.Println(x)
}

// Output:
// after the first blank label
// 6
