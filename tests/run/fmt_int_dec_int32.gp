// phase: 05

// Go's %d of int32 values with every flag, width and precision: the integer formatting of
// fmt/format.go.
package main

import "fmt"

func main() {
	fmt.Printf("[%d]\n", int32(2147483647))
	fmt.Printf("[%+d]\n", int32(2147483647))
	fmt.Printf("[%-d]\n", int32(2147483647))
	fmt.Printf("[% d]\n", int32(2147483647))
	fmt.Printf("[%0d]\n", int32(2147483647))
	fmt.Printf("[%#d]\n", int32(2147483647))
	fmt.Printf("[%8d]\n", int32(2147483647))
	fmt.Printf("[%-8d]\n", int32(2147483647))
	fmt.Printf("[%08d]\n", int32(2147483647))
	fmt.Printf("[%+8d]\n", int32(2147483647))
	fmt.Printf("[%+08d]\n", int32(2147483647))
	fmt.Printf("[% 8d]\n", int32(2147483647))
	fmt.Printf("[%8.3d]\n", int32(2147483647))
	fmt.Printf("[%.3d]\n", int32(2147483647))
	fmt.Printf("[%.0d]\n", int32(2147483647))
	fmt.Printf("[%#8d]\n", int32(2147483647))
	fmt.Printf("[%#08d]\n", int32(2147483647))
	fmt.Printf("[%-#8d]\n", int32(2147483647))
	fmt.Printf("[%+.5d]\n", int32(2147483647))
	fmt.Printf("[%0-8d]\n", int32(2147483647))
	fmt.Printf("[%12.8d]\n", int32(2147483647))
	fmt.Printf("[%-12.8d]\n", int32(2147483647))
	fmt.Printf("[%#.4d]\n", int32(2147483647))
	fmt.Printf("[%+#d]\n", int32(2147483647))
	fmt.Printf("[%d]\n", int32(-2147483648))
	fmt.Printf("[%+d]\n", int32(-2147483648))
	fmt.Printf("[%-d]\n", int32(-2147483648))
	fmt.Printf("[% d]\n", int32(-2147483648))
	fmt.Printf("[%0d]\n", int32(-2147483648))
	fmt.Printf("[%#d]\n", int32(-2147483648))
	fmt.Printf("[%8d]\n", int32(-2147483648))
	fmt.Printf("[%-8d]\n", int32(-2147483648))
	fmt.Printf("[%08d]\n", int32(-2147483648))
	fmt.Printf("[%+8d]\n", int32(-2147483648))
	fmt.Printf("[%+08d]\n", int32(-2147483648))
	fmt.Printf("[% 8d]\n", int32(-2147483648))
	fmt.Printf("[%8.3d]\n", int32(-2147483648))
	fmt.Printf("[%.3d]\n", int32(-2147483648))
	fmt.Printf("[%.0d]\n", int32(-2147483648))
	fmt.Printf("[%#8d]\n", int32(-2147483648))
	fmt.Printf("[%#08d]\n", int32(-2147483648))
	fmt.Printf("[%-#8d]\n", int32(-2147483648))
	fmt.Printf("[%+.5d]\n", int32(-2147483648))
	fmt.Printf("[%0-8d]\n", int32(-2147483648))
	fmt.Printf("[%12.8d]\n", int32(-2147483648))
	fmt.Printf("[%-12.8d]\n", int32(-2147483648))
	fmt.Printf("[%#.4d]\n", int32(-2147483648))
	fmt.Printf("[%+#d]\n", int32(-2147483648))
	fmt.Printf("[%d]\n", int32(65))
	fmt.Printf("[%+d]\n", int32(65))
	fmt.Printf("[%-d]\n", int32(65))
	fmt.Printf("[% d]\n", int32(65))
	fmt.Printf("[%0d]\n", int32(65))
	fmt.Printf("[%#d]\n", int32(65))
	fmt.Printf("[%8d]\n", int32(65))
	fmt.Printf("[%-8d]\n", int32(65))
	fmt.Printf("[%08d]\n", int32(65))
	fmt.Printf("[%+8d]\n", int32(65))
	fmt.Printf("[%+08d]\n", int32(65))
	fmt.Printf("[% 8d]\n", int32(65))
	fmt.Printf("[%8.3d]\n", int32(65))
	fmt.Printf("[%.3d]\n", int32(65))
	fmt.Printf("[%.0d]\n", int32(65))
	fmt.Printf("[%#8d]\n", int32(65))
	fmt.Printf("[%#08d]\n", int32(65))
	fmt.Printf("[%-#8d]\n", int32(65))
	fmt.Printf("[%+.5d]\n", int32(65))
	fmt.Printf("[%0-8d]\n", int32(65))
	fmt.Printf("[%12.8d]\n", int32(65))
	fmt.Printf("[%-12.8d]\n", int32(65))
	fmt.Printf("[%#.4d]\n", int32(65))
	fmt.Printf("[%+#d]\n", int32(65))
}

// Output:
// [2147483647]
// [+2147483647]
// [2147483647]
// [ 2147483647]
// [2147483647]
// [2147483647]
// [2147483647]
// [2147483647]
// [2147483647]
// [+2147483647]
// [+2147483647]
// [ 2147483647]
// [2147483647]
// [2147483647]
// [2147483647]
// [2147483647]
// [2147483647]
// [2147483647]
// [+2147483647]
// [2147483647]
// [  2147483647]
// [2147483647  ]
// [2147483647]
// [+2147483647]
// [-2147483648]
// [-2147483648]
// [-2147483648]
// [-2147483648]
// [-2147483648]
// [-2147483648]
// [-2147483648]
// [-2147483648]
// [-2147483648]
// [-2147483648]
// [-2147483648]
// [-2147483648]
// [-2147483648]
// [-2147483648]
// [-2147483648]
// [-2147483648]
// [-2147483648]
// [-2147483648]
// [-2147483648]
// [-2147483648]
// [ -2147483648]
// [-2147483648 ]
// [-2147483648]
// [-2147483648]
// [65]
// [+65]
// [65]
// [ 65]
// [65]
// [65]
// [      65]
// [65      ]
// [00000065]
// [     +65]
// [+0000065]
// [      65]
// [     065]
// [065]
// [65]
// [      65]
// [00000065]
// [65      ]
// [+00065]
// [65      ]
// [    00000065]
// [00000065    ]
// [0065]
// [+65]
