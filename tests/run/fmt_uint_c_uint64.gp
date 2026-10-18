// phase: 05

// Go's %c of uint64 values with flags, width and precision.
package main

import "fmt"

func main() {
	fmt.Printf("[%c]\n", uint64(18446744073709551615))
	fmt.Printf("[%8c]\n", uint64(18446744073709551615))
	fmt.Printf("[%-8c]\n", uint64(18446744073709551615))
	fmt.Printf("[%c]\n", uint64(1)<<40)
	fmt.Printf("[%8c]\n", uint64(1)<<40)
	fmt.Printf("[%-8c]\n", uint64(1)<<40)
}

// Output:
// [�]
// [       �]
// [�       ]
// [�]
// [       �]
// [�       ]
