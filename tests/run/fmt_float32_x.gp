// phase: 05

// Go's %x of float32 values: the shortest digits are those of a float32 (0.1, not
// 0.10000000149011612).
package main

import "fmt"

func main() {
	fmt.Printf("[%x]\n", float32(0.1))
	fmt.Printf("[%+x]\n", float32(0.1))
	fmt.Printf("[%10x]\n", float32(0.1))
	fmt.Printf("[%-10x]\n", float32(0.1))
	fmt.Printf("[%.3x]\n", float32(0.1))
	fmt.Printf("[%#x]\n", float32(0.1))
	fmt.Printf("[%010.2x]\n", float32(0.1))
	fmt.Printf("[%x]\n", float32(3.14159274))
	fmt.Printf("[%+x]\n", float32(3.14159274))
	fmt.Printf("[%10x]\n", float32(3.14159274))
	fmt.Printf("[%-10x]\n", float32(3.14159274))
	fmt.Printf("[%.3x]\n", float32(3.14159274))
	fmt.Printf("[%#x]\n", float32(3.14159274))
	fmt.Printf("[%010.2x]\n", float32(3.14159274))
	fmt.Printf("[%x]\n", float32(16777216.0))
	fmt.Printf("[%+x]\n", float32(16777216.0))
	fmt.Printf("[%10x]\n", float32(16777216.0))
	fmt.Printf("[%-10x]\n", float32(16777216.0))
	fmt.Printf("[%.3x]\n", float32(16777216.0))
	fmt.Printf("[%#x]\n", float32(16777216.0))
	fmt.Printf("[%010.2x]\n", float32(16777216.0))
	fmt.Printf("[%x]\n", float32(1e-7))
	fmt.Printf("[%+x]\n", float32(1e-7))
	fmt.Printf("[%10x]\n", float32(1e-7))
	fmt.Printf("[%-10x]\n", float32(1e-7))
	fmt.Printf("[%.3x]\n", float32(1e-7))
	fmt.Printf("[%#x]\n", float32(1e-7))
	fmt.Printf("[%010.2x]\n", float32(1e-7))
	fmt.Printf("[%x]\n", float32(1e20))
	fmt.Printf("[%+x]\n", float32(1e20))
	fmt.Printf("[%10x]\n", float32(1e20))
	fmt.Printf("[%-10x]\n", float32(1e20))
	fmt.Printf("[%.3x]\n", float32(1e20))
	fmt.Printf("[%#x]\n", float32(1e20))
	fmt.Printf("[%010.2x]\n", float32(1e20))
	fmt.Printf("[%x]\n", float32(2.5))
	fmt.Printf("[%+x]\n", float32(2.5))
	fmt.Printf("[%10x]\n", float32(2.5))
	fmt.Printf("[%-10x]\n", float32(2.5))
	fmt.Printf("[%.3x]\n", float32(2.5))
	fmt.Printf("[%#x]\n", float32(2.5))
	fmt.Printf("[%010.2x]\n", float32(2.5))
	fmt.Printf("[%x]\n", float32(-0.0))
	fmt.Printf("[%+x]\n", float32(-0.0))
	fmt.Printf("[%10x]\n", float32(-0.0))
	fmt.Printf("[%-10x]\n", float32(-0.0))
	fmt.Printf("[%.3x]\n", float32(-0.0))
	fmt.Printf("[%#x]\n", float32(-0.0))
	fmt.Printf("[%010.2x]\n", float32(-0.0))
	fmt.Printf("[%x]\n", float32(1)/3)
	fmt.Printf("[%+x]\n", float32(1)/3)
	fmt.Printf("[%10x]\n", float32(1)/3)
	fmt.Printf("[%-10x]\n", float32(1)/3)
	fmt.Printf("[%.3x]\n", float32(1)/3)
	fmt.Printf("[%#x]\n", float32(1)/3)
	fmt.Printf("[%010.2x]\n", float32(1)/3)
}

// Output:
// [0x1.99999ap-04]
// [+0x1.99999ap-04]
// [0x1.99999ap-04]
// [0x1.99999ap-04]
// [0x1.99ap-04]
// [0x1.99999ap-04]
// [0x1.9ap-04]
// [0x1.921fb6p+01]
// [+0x1.921fb6p+01]
// [0x1.921fb6p+01]
// [0x1.921fb6p+01]
// [0x1.922p+01]
// [0x1.921fb6p+01]
// [0x1.92p+01]
// [0x1p+24]
// [+0x1p+24]
// [   0x1p+24]
// [0x1p+24   ]
// [0x1.000p+24]
// [0x1.0000p+24]
// [0x1.00p+24]
// [0x1.ad7f2ap-24]
// [+0x1.ad7f2ap-24]
// [0x1.ad7f2ap-24]
// [0x1.ad7f2ap-24]
// [0x1.ad8p-24]
// [0x1.ad7f2ap-24]
// [0x1.adp-24]
// [0x1.5af1d8p+66]
// [+0x1.5af1d8p+66]
// [0x1.5af1d8p+66]
// [0x1.5af1d8p+66]
// [0x1.5afp+66]
// [0x1.5af1d8p+66]
// [0x1.5bp+66]
// [0x1.4p+01]
// [+0x1.4p+01]
// [ 0x1.4p+01]
// [0x1.4p+01 ]
// [0x1.400p+01]
// [0x1.4000p+01]
// [0x1.40p+01]
// [0x0p+00]
// [+0x0p+00]
// [   0x0p+00]
// [0x0p+00   ]
// [0x0.000p+00]
// [0x0.0000p+00]
// [0x0.00p+00]
// [0x1.555556p-02]
// [+0x1.555556p-02]
// [0x1.555556p-02]
// [0x1.555556p-02]
// [0x1.555p-02]
// [0x1.555556p-02]
// [0x1.55p-02]
