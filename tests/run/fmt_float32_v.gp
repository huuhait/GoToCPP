// phase: 05

// Go's %v of float32 values: the shortest digits are those of a float32 (0.1, not
// 0.10000000149011612).
package main

import "fmt"

func main() {
	fmt.Printf("[%v]\n", float32(0.1))
	fmt.Printf("[%+v]\n", float32(0.1))
	fmt.Printf("[%10v]\n", float32(0.1))
	fmt.Printf("[%-10v]\n", float32(0.1))
	fmt.Printf("[%.3v]\n", float32(0.1))
	fmt.Printf("[%#v]\n", float32(0.1))
	fmt.Printf("[%010.2v]\n", float32(0.1))
	fmt.Printf("[%v]\n", float32(3.14159274))
	fmt.Printf("[%+v]\n", float32(3.14159274))
	fmt.Printf("[%10v]\n", float32(3.14159274))
	fmt.Printf("[%-10v]\n", float32(3.14159274))
	fmt.Printf("[%.3v]\n", float32(3.14159274))
	fmt.Printf("[%#v]\n", float32(3.14159274))
	fmt.Printf("[%010.2v]\n", float32(3.14159274))
	fmt.Printf("[%v]\n", float32(16777216.0))
	fmt.Printf("[%+v]\n", float32(16777216.0))
	fmt.Printf("[%10v]\n", float32(16777216.0))
	fmt.Printf("[%-10v]\n", float32(16777216.0))
	fmt.Printf("[%.3v]\n", float32(16777216.0))
	fmt.Printf("[%#v]\n", float32(16777216.0))
	fmt.Printf("[%010.2v]\n", float32(16777216.0))
	fmt.Printf("[%v]\n", float32(1e-7))
	fmt.Printf("[%+v]\n", float32(1e-7))
	fmt.Printf("[%10v]\n", float32(1e-7))
	fmt.Printf("[%-10v]\n", float32(1e-7))
	fmt.Printf("[%.3v]\n", float32(1e-7))
	fmt.Printf("[%#v]\n", float32(1e-7))
	fmt.Printf("[%010.2v]\n", float32(1e-7))
	fmt.Printf("[%v]\n", float32(1e20))
	fmt.Printf("[%+v]\n", float32(1e20))
	fmt.Printf("[%10v]\n", float32(1e20))
	fmt.Printf("[%-10v]\n", float32(1e20))
	fmt.Printf("[%.3v]\n", float32(1e20))
	fmt.Printf("[%#v]\n", float32(1e20))
	fmt.Printf("[%010.2v]\n", float32(1e20))
	fmt.Printf("[%v]\n", float32(2.5))
	fmt.Printf("[%+v]\n", float32(2.5))
	fmt.Printf("[%10v]\n", float32(2.5))
	fmt.Printf("[%-10v]\n", float32(2.5))
	fmt.Printf("[%.3v]\n", float32(2.5))
	fmt.Printf("[%#v]\n", float32(2.5))
	fmt.Printf("[%010.2v]\n", float32(2.5))
	fmt.Printf("[%v]\n", float32(-0.0))
	fmt.Printf("[%+v]\n", float32(-0.0))
	fmt.Printf("[%10v]\n", float32(-0.0))
	fmt.Printf("[%-10v]\n", float32(-0.0))
	fmt.Printf("[%.3v]\n", float32(-0.0))
	fmt.Printf("[%#v]\n", float32(-0.0))
	fmt.Printf("[%010.2v]\n", float32(-0.0))
	fmt.Printf("[%v]\n", float32(1)/3)
	fmt.Printf("[%+v]\n", float32(1)/3)
	fmt.Printf("[%10v]\n", float32(1)/3)
	fmt.Printf("[%-10v]\n", float32(1)/3)
	fmt.Printf("[%.3v]\n", float32(1)/3)
	fmt.Printf("[%#v]\n", float32(1)/3)
	fmt.Printf("[%010.2v]\n", float32(1)/3)
}

// Output:
// [0.1]
// [0.1]
// [       0.1]
// [0.1       ]
// [0.1]
// [0.1]
// [00000000.1]
// [3.1415927]
// [3.1415927]
// [ 3.1415927]
// [3.1415927 ]
// [3.14]
// [3.1415927]
// [00000003.1]
// [1.6777216e+07]
// [1.6777216e+07]
// [1.6777216e+07]
// [1.6777216e+07]
// [1.68e+07]
// [1.6777216e+07]
// [0001.7e+07]
// [1e-07]
// [1e-07]
// [     1e-07]
// [1e-07     ]
// [1e-07]
// [1e-07]
// [000001e-07]
// [1e+20]
// [1e+20]
// [     1e+20]
// [1e+20     ]
// [1e+20]
// [1e+20]
// [000001e+20]
// [2.5]
// [2.5]
// [       2.5]
// [2.5       ]
// [2.5]
// [2.5]
// [00000002.5]
// [0]
// [0]
// [         0]
// [0         ]
// [0]
// [0]
// [0000000000]
// [0.33333334]
// [0.33333334]
// [0.33333334]
// [0.33333334]
// [0.333]
// [0.33333334]
// [0000000.33]
