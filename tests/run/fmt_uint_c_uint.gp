// phase: 05

// Go's %c of uint values with flags, width and precision.
package main

import "fmt"

func main() {
	fmt.Printf("[%c]\n", uint(1))
	fmt.Printf("[%8c]\n", uint(1))
	fmt.Printf("[%-8c]\n", uint(1))
	fmt.Printf("[%c]\n", uint(42))
	fmt.Printf("[%8c]\n", uint(42))
	fmt.Printf("[%-8c]\n", uint(42))
	fmt.Printf("[%c]\n", uint(1234567))
	fmt.Printf("[%8c]\n", uint(1234567))
	fmt.Printf("[%-8c]\n", uint(1234567))
}

// Output:
// []
// [       ]
// [       ]
// [*]
// [       *]
// [*       ]
// [�]
// [       �]
// [�       ]
