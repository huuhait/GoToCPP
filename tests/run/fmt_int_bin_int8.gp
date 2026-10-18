// phase: 05

// Go's %b of int8 values with every flag, width and precision: the integer formatting of
// fmt/format.go.
package main

import "fmt"

func main() {
	fmt.Printf("[%b]\n", int8(0))
	fmt.Printf("[%+b]\n", int8(0))
	fmt.Printf("[%-b]\n", int8(0))
	fmt.Printf("[% b]\n", int8(0))
	fmt.Printf("[%0b]\n", int8(0))
	fmt.Printf("[%#b]\n", int8(0))
	fmt.Printf("[%8b]\n", int8(0))
	fmt.Printf("[%-8b]\n", int8(0))
	fmt.Printf("[%08b]\n", int8(0))
	fmt.Printf("[%+8b]\n", int8(0))
	fmt.Printf("[%+08b]\n", int8(0))
	fmt.Printf("[% 8b]\n", int8(0))
	fmt.Printf("[%8.3b]\n", int8(0))
	fmt.Printf("[%.3b]\n", int8(0))
	fmt.Printf("[%.0b]\n", int8(0))
	fmt.Printf("[%#8b]\n", int8(0))
	fmt.Printf("[%#08b]\n", int8(0))
	fmt.Printf("[%-#8b]\n", int8(0))
	fmt.Printf("[%+.5b]\n", int8(0))
	fmt.Printf("[%0-8b]\n", int8(0))
	fmt.Printf("[%12.8b]\n", int8(0))
	fmt.Printf("[%-12.8b]\n", int8(0))
	fmt.Printf("[%#.4b]\n", int8(0))
	fmt.Printf("[%+#b]\n", int8(0))
	fmt.Printf("[%b]\n", int8(127))
	fmt.Printf("[%+b]\n", int8(127))
	fmt.Printf("[%-b]\n", int8(127))
	fmt.Printf("[% b]\n", int8(127))
	fmt.Printf("[%0b]\n", int8(127))
	fmt.Printf("[%#b]\n", int8(127))
	fmt.Printf("[%8b]\n", int8(127))
	fmt.Printf("[%-8b]\n", int8(127))
	fmt.Printf("[%08b]\n", int8(127))
	fmt.Printf("[%+8b]\n", int8(127))
	fmt.Printf("[%+08b]\n", int8(127))
	fmt.Printf("[% 8b]\n", int8(127))
	fmt.Printf("[%8.3b]\n", int8(127))
	fmt.Printf("[%.3b]\n", int8(127))
	fmt.Printf("[%.0b]\n", int8(127))
	fmt.Printf("[%#8b]\n", int8(127))
	fmt.Printf("[%#08b]\n", int8(127))
	fmt.Printf("[%-#8b]\n", int8(127))
	fmt.Printf("[%+.5b]\n", int8(127))
	fmt.Printf("[%0-8b]\n", int8(127))
	fmt.Printf("[%12.8b]\n", int8(127))
	fmt.Printf("[%-12.8b]\n", int8(127))
	fmt.Printf("[%#.4b]\n", int8(127))
	fmt.Printf("[%+#b]\n", int8(127))
	fmt.Printf("[%b]\n", int8(-128))
	fmt.Printf("[%+b]\n", int8(-128))
	fmt.Printf("[%-b]\n", int8(-128))
	fmt.Printf("[% b]\n", int8(-128))
	fmt.Printf("[%0b]\n", int8(-128))
	fmt.Printf("[%#b]\n", int8(-128))
	fmt.Printf("[%8b]\n", int8(-128))
	fmt.Printf("[%-8b]\n", int8(-128))
	fmt.Printf("[%08b]\n", int8(-128))
	fmt.Printf("[%+8b]\n", int8(-128))
	fmt.Printf("[%+08b]\n", int8(-128))
	fmt.Printf("[% 8b]\n", int8(-128))
	fmt.Printf("[%8.3b]\n", int8(-128))
	fmt.Printf("[%.3b]\n", int8(-128))
	fmt.Printf("[%.0b]\n", int8(-128))
	fmt.Printf("[%#8b]\n", int8(-128))
	fmt.Printf("[%#08b]\n", int8(-128))
	fmt.Printf("[%-#8b]\n", int8(-128))
	fmt.Printf("[%+.5b]\n", int8(-128))
	fmt.Printf("[%0-8b]\n", int8(-128))
	fmt.Printf("[%12.8b]\n", int8(-128))
	fmt.Printf("[%-12.8b]\n", int8(-128))
	fmt.Printf("[%#.4b]\n", int8(-128))
	fmt.Printf("[%+#b]\n", int8(-128))
	fmt.Printf("[%b]\n", int8(-5))
	fmt.Printf("[%+b]\n", int8(-5))
	fmt.Printf("[%-b]\n", int8(-5))
	fmt.Printf("[% b]\n", int8(-5))
	fmt.Printf("[%0b]\n", int8(-5))
	fmt.Printf("[%#b]\n", int8(-5))
	fmt.Printf("[%8b]\n", int8(-5))
	fmt.Printf("[%-8b]\n", int8(-5))
	fmt.Printf("[%08b]\n", int8(-5))
	fmt.Printf("[%+8b]\n", int8(-5))
	fmt.Printf("[%+08b]\n", int8(-5))
	fmt.Printf("[% 8b]\n", int8(-5))
	fmt.Printf("[%8.3b]\n", int8(-5))
	fmt.Printf("[%.3b]\n", int8(-5))
	fmt.Printf("[%.0b]\n", int8(-5))
	fmt.Printf("[%#8b]\n", int8(-5))
	fmt.Printf("[%#08b]\n", int8(-5))
	fmt.Printf("[%-#8b]\n", int8(-5))
	fmt.Printf("[%+.5b]\n", int8(-5))
	fmt.Printf("[%0-8b]\n", int8(-5))
	fmt.Printf("[%12.8b]\n", int8(-5))
	fmt.Printf("[%-12.8b]\n", int8(-5))
	fmt.Printf("[%#.4b]\n", int8(-5))
	fmt.Printf("[%+#b]\n", int8(-5))
}

// Output:
// [0]
// [+0]
// [0]
// [ 0]
// [0]
// [0b0]
// [       0]
// [0       ]
// [00000000]
// [      +0]
// [+0000000]
// [       0]
// [     000]
// [000]
// []
// [     0b0]
// [0b00000000]
// [0b0     ]
// [+00000]
// [0       ]
// [    00000000]
// [00000000    ]
// [0b0000]
// [+0b0]
// [1111111]
// [+1111111]
// [1111111]
// [ 1111111]
// [1111111]
// [0b1111111]
// [ 1111111]
// [1111111 ]
// [01111111]
// [+1111111]
// [+1111111]
// [ 1111111]
// [ 1111111]
// [1111111]
// [1111111]
// [0b1111111]
// [0b01111111]
// [0b1111111]
// [+1111111]
// [1111111 ]
// [    01111111]
// [01111111    ]
// [0b1111111]
// [+0b1111111]
// [-10000000]
// [-10000000]
// [-10000000]
// [-10000000]
// [-10000000]
// [-0b10000000]
// [-10000000]
// [-10000000]
// [-10000000]
// [-10000000]
// [-10000000]
// [-10000000]
// [-10000000]
// [-10000000]
// [-10000000]
// [-0b10000000]
// [-0b10000000]
// [-0b10000000]
// [-10000000]
// [-10000000]
// [   -10000000]
// [-10000000   ]
// [-0b10000000]
// [-0b10000000]
// [-101]
// [-101]
// [-101]
// [-101]
// [-101]
// [-0b101]
// [    -101]
// [-101    ]
// [-0000101]
// [    -101]
// [-0000101]
// [    -101]
// [    -101]
// [-101]
// [-101]
// [  -0b101]
// [-0b0000101]
// [-0b101  ]
// [-00101]
// [-101    ]
// [   -00000101]
// [-00000101   ]
// [-0b0101]
// [-0b101]
