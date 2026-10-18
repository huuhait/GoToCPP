// phase: 05

// Go's %f of float32 values: the shortest digits are those of a float32 (0.1, not
// 0.10000000149011612).
package main

import "fmt"

func main() {
	fmt.Printf("[%f]\n", float32(0.1))
	fmt.Printf("[%+f]\n", float32(0.1))
	fmt.Printf("[%10f]\n", float32(0.1))
	fmt.Printf("[%-10f]\n", float32(0.1))
	fmt.Printf("[%.3f]\n", float32(0.1))
	fmt.Printf("[%#f]\n", float32(0.1))
	fmt.Printf("[%010.2f]\n", float32(0.1))
	fmt.Printf("[%f]\n", float32(3.14159274))
	fmt.Printf("[%+f]\n", float32(3.14159274))
	fmt.Printf("[%10f]\n", float32(3.14159274))
	fmt.Printf("[%-10f]\n", float32(3.14159274))
	fmt.Printf("[%.3f]\n", float32(3.14159274))
	fmt.Printf("[%#f]\n", float32(3.14159274))
	fmt.Printf("[%010.2f]\n", float32(3.14159274))
	fmt.Printf("[%f]\n", float32(16777216.0))
	fmt.Printf("[%+f]\n", float32(16777216.0))
	fmt.Printf("[%10f]\n", float32(16777216.0))
	fmt.Printf("[%-10f]\n", float32(16777216.0))
	fmt.Printf("[%.3f]\n", float32(16777216.0))
	fmt.Printf("[%#f]\n", float32(16777216.0))
	fmt.Printf("[%010.2f]\n", float32(16777216.0))
	fmt.Printf("[%f]\n", float32(1e-7))
	fmt.Printf("[%+f]\n", float32(1e-7))
	fmt.Printf("[%10f]\n", float32(1e-7))
	fmt.Printf("[%-10f]\n", float32(1e-7))
	fmt.Printf("[%.3f]\n", float32(1e-7))
	fmt.Printf("[%#f]\n", float32(1e-7))
	fmt.Printf("[%010.2f]\n", float32(1e-7))
	fmt.Printf("[%f]\n", float32(1e20))
	fmt.Printf("[%+f]\n", float32(1e20))
	fmt.Printf("[%10f]\n", float32(1e20))
	fmt.Printf("[%-10f]\n", float32(1e20))
	fmt.Printf("[%.3f]\n", float32(1e20))
	fmt.Printf("[%#f]\n", float32(1e20))
	fmt.Printf("[%010.2f]\n", float32(1e20))
	fmt.Printf("[%f]\n", float32(2.5))
	fmt.Printf("[%+f]\n", float32(2.5))
	fmt.Printf("[%10f]\n", float32(2.5))
	fmt.Printf("[%-10f]\n", float32(2.5))
	fmt.Printf("[%.3f]\n", float32(2.5))
	fmt.Printf("[%#f]\n", float32(2.5))
	fmt.Printf("[%010.2f]\n", float32(2.5))
	fmt.Printf("[%f]\n", float32(-0.0))
	fmt.Printf("[%+f]\n", float32(-0.0))
	fmt.Printf("[%10f]\n", float32(-0.0))
	fmt.Printf("[%-10f]\n", float32(-0.0))
	fmt.Printf("[%.3f]\n", float32(-0.0))
	fmt.Printf("[%#f]\n", float32(-0.0))
	fmt.Printf("[%010.2f]\n", float32(-0.0))
	fmt.Printf("[%f]\n", float32(1)/3)
	fmt.Printf("[%+f]\n", float32(1)/3)
	fmt.Printf("[%10f]\n", float32(1)/3)
	fmt.Printf("[%-10f]\n", float32(1)/3)
	fmt.Printf("[%.3f]\n", float32(1)/3)
	fmt.Printf("[%#f]\n", float32(1)/3)
	fmt.Printf("[%010.2f]\n", float32(1)/3)
}

// Output:
// [0.100000]
// [+0.100000]
// [  0.100000]
// [0.100000  ]
// [0.100]
// [0.100000]
// [0000000.10]
// [3.141593]
// [+3.141593]
// [  3.141593]
// [3.141593  ]
// [3.142]
// [3.141593]
// [0000003.14]
// [16777216.000000]
// [+16777216.000000]
// [16777216.000000]
// [16777216.000000]
// [16777216.000]
// [16777216.000000]
// [16777216.00]
// [0.000000]
// [+0.000000]
// [  0.000000]
// [0.000000  ]
// [0.000]
// [0.000000]
// [0000000.00]
// [100000002004087734272.000000]
// [+100000002004087734272.000000]
// [100000002004087734272.000000]
// [100000002004087734272.000000]
// [100000002004087734272.000]
// [100000002004087734272.000000]
// [100000002004087734272.00]
// [2.500000]
// [+2.500000]
// [  2.500000]
// [2.500000  ]
// [2.500]
// [2.500000]
// [0000002.50]
// [0.000000]
// [+0.000000]
// [  0.000000]
// [0.000000  ]
// [0.000]
// [0.000000]
// [0000000.00]
// [0.333333]
// [+0.333333]
// [  0.333333]
// [0.333333  ]
// [0.333]
// [0.333333]
// [0000000.33]
