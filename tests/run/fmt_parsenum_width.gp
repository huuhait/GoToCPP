// phase: 09

package main

import "fmt"

func main() {
	var a, b int
	n, err := fmt.Sscanf("5 6", "%99999999d %d", &a, &b)
	fmt.Println(n, err, a, b)
	a, b = 0, 0
	n, err = fmt.Sscanf("5 6", "%9999999d %d", &a, &b)
	fmt.Println(n, err, a, b)
}

// Output:
// 1 too many operands 5 0
// 2 <nil> 5 6
