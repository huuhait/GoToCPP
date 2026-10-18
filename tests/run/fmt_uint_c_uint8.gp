// phase: 05

// Go's %c of uint8 values with flags, width and precision.
package main

import "fmt"

func main() {
	fmt.Printf("[%c]\n", uint8(255))
	fmt.Printf("[%8c]\n", uint8(255))
	fmt.Printf("[%-8c]\n", uint8(255))
	fmt.Printf("[%c]\n", uint8(65))
	fmt.Printf("[%8c]\n", uint8(65))
	fmt.Printf("[%-8c]\n", uint8(65))
}

// Output:
// [ÿ]
// [       ÿ]
// [ÿ       ]
// [A]
// [       A]
// [A       ]
