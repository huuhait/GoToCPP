// phase: 05

// Go's %x of int8 values with every flag, width and precision: the integer formatting of
// fmt/format.go.
package main

import "fmt"

func main() {
	fmt.Printf("[%x]\n", int8(0))
	fmt.Printf("[%+x]\n", int8(0))
	fmt.Printf("[%-x]\n", int8(0))
	fmt.Printf("[% x]\n", int8(0))
	fmt.Printf("[%0x]\n", int8(0))
	fmt.Printf("[%#x]\n", int8(0))
	fmt.Printf("[%8x]\n", int8(0))
	fmt.Printf("[%-8x]\n", int8(0))
	fmt.Printf("[%08x]\n", int8(0))
	fmt.Printf("[%+8x]\n", int8(0))
	fmt.Printf("[%+08x]\n", int8(0))
	fmt.Printf("[% 8x]\n", int8(0))
	fmt.Printf("[%8.3x]\n", int8(0))
	fmt.Printf("[%.3x]\n", int8(0))
	fmt.Printf("[%.0x]\n", int8(0))
	fmt.Printf("[%#8x]\n", int8(0))
	fmt.Printf("[%#08x]\n", int8(0))
	fmt.Printf("[%-#8x]\n", int8(0))
	fmt.Printf("[%+.5x]\n", int8(0))
	fmt.Printf("[%0-8x]\n", int8(0))
	fmt.Printf("[%12.8x]\n", int8(0))
	fmt.Printf("[%-12.8x]\n", int8(0))
	fmt.Printf("[%#.4x]\n", int8(0))
	fmt.Printf("[%+#x]\n", int8(0))
	fmt.Printf("[%x]\n", int8(127))
	fmt.Printf("[%+x]\n", int8(127))
	fmt.Printf("[%-x]\n", int8(127))
	fmt.Printf("[% x]\n", int8(127))
	fmt.Printf("[%0x]\n", int8(127))
	fmt.Printf("[%#x]\n", int8(127))
	fmt.Printf("[%8x]\n", int8(127))
	fmt.Printf("[%-8x]\n", int8(127))
	fmt.Printf("[%08x]\n", int8(127))
	fmt.Printf("[%+8x]\n", int8(127))
	fmt.Printf("[%+08x]\n", int8(127))
	fmt.Printf("[% 8x]\n", int8(127))
	fmt.Printf("[%8.3x]\n", int8(127))
	fmt.Printf("[%.3x]\n", int8(127))
	fmt.Printf("[%.0x]\n", int8(127))
	fmt.Printf("[%#8x]\n", int8(127))
	fmt.Printf("[%#08x]\n", int8(127))
	fmt.Printf("[%-#8x]\n", int8(127))
	fmt.Printf("[%+.5x]\n", int8(127))
	fmt.Printf("[%0-8x]\n", int8(127))
	fmt.Printf("[%12.8x]\n", int8(127))
	fmt.Printf("[%-12.8x]\n", int8(127))
	fmt.Printf("[%#.4x]\n", int8(127))
	fmt.Printf("[%+#x]\n", int8(127))
	fmt.Printf("[%x]\n", int8(-128))
	fmt.Printf("[%+x]\n", int8(-128))
	fmt.Printf("[%-x]\n", int8(-128))
	fmt.Printf("[% x]\n", int8(-128))
	fmt.Printf("[%0x]\n", int8(-128))
	fmt.Printf("[%#x]\n", int8(-128))
	fmt.Printf("[%8x]\n", int8(-128))
	fmt.Printf("[%-8x]\n", int8(-128))
	fmt.Printf("[%08x]\n", int8(-128))
	fmt.Printf("[%+8x]\n", int8(-128))
	fmt.Printf("[%+08x]\n", int8(-128))
	fmt.Printf("[% 8x]\n", int8(-128))
	fmt.Printf("[%8.3x]\n", int8(-128))
	fmt.Printf("[%.3x]\n", int8(-128))
	fmt.Printf("[%.0x]\n", int8(-128))
	fmt.Printf("[%#8x]\n", int8(-128))
	fmt.Printf("[%#08x]\n", int8(-128))
	fmt.Printf("[%-#8x]\n", int8(-128))
	fmt.Printf("[%+.5x]\n", int8(-128))
	fmt.Printf("[%0-8x]\n", int8(-128))
	fmt.Printf("[%12.8x]\n", int8(-128))
	fmt.Printf("[%-12.8x]\n", int8(-128))
	fmt.Printf("[%#.4x]\n", int8(-128))
	fmt.Printf("[%+#x]\n", int8(-128))
	fmt.Printf("[%x]\n", int8(-5))
	fmt.Printf("[%+x]\n", int8(-5))
	fmt.Printf("[%-x]\n", int8(-5))
	fmt.Printf("[% x]\n", int8(-5))
	fmt.Printf("[%0x]\n", int8(-5))
	fmt.Printf("[%#x]\n", int8(-5))
	fmt.Printf("[%8x]\n", int8(-5))
	fmt.Printf("[%-8x]\n", int8(-5))
	fmt.Printf("[%08x]\n", int8(-5))
	fmt.Printf("[%+8x]\n", int8(-5))
	fmt.Printf("[%+08x]\n", int8(-5))
	fmt.Printf("[% 8x]\n", int8(-5))
	fmt.Printf("[%8.3x]\n", int8(-5))
	fmt.Printf("[%.3x]\n", int8(-5))
	fmt.Printf("[%.0x]\n", int8(-5))
	fmt.Printf("[%#8x]\n", int8(-5))
	fmt.Printf("[%#08x]\n", int8(-5))
	fmt.Printf("[%-#8x]\n", int8(-5))
	fmt.Printf("[%+.5x]\n", int8(-5))
	fmt.Printf("[%0-8x]\n", int8(-5))
	fmt.Printf("[%12.8x]\n", int8(-5))
	fmt.Printf("[%-12.8x]\n", int8(-5))
	fmt.Printf("[%#.4x]\n", int8(-5))
	fmt.Printf("[%+#x]\n", int8(-5))
}

// Output:
// [0]
// [+0]
// [0]
// [ 0]
// [0]
// [0x0]
// [       0]
// [0       ]
// [00000000]
// [      +0]
// [+0000000]
// [       0]
// [     000]
// [000]
// []
// [     0x0]
// [0x00000000]
// [0x0     ]
// [+00000]
// [0       ]
// [    00000000]
// [00000000    ]
// [0x0000]
// [+0x0]
// [7f]
// [+7f]
// [7f]
// [ 7f]
// [7f]
// [0x7f]
// [      7f]
// [7f      ]
// [0000007f]
// [     +7f]
// [+000007f]
// [      7f]
// [     07f]
// [07f]
// [7f]
// [    0x7f]
// [0x0000007f]
// [0x7f    ]
// [+0007f]
// [7f      ]
// [    0000007f]
// [0000007f    ]
// [0x007f]
// [+0x7f]
// [-80]
// [-80]
// [-80]
// [-80]
// [-80]
// [-0x80]
// [     -80]
// [-80     ]
// [-0000080]
// [     -80]
// [-0000080]
// [     -80]
// [    -080]
// [-080]
// [-80]
// [   -0x80]
// [-0x0000080]
// [-0x80   ]
// [-00080]
// [-80     ]
// [   -00000080]
// [-00000080   ]
// [-0x0080]
// [-0x80]
// [-5]
// [-5]
// [-5]
// [-5]
// [-5]
// [-0x5]
// [      -5]
// [-5      ]
// [-0000005]
// [      -5]
// [-0000005]
// [      -5]
// [    -005]
// [-005]
// [-5]
// [    -0x5]
// [-0x0000005]
// [-0x5    ]
// [-00005]
// [-5      ]
// [   -00000005]
// [-00000005   ]
// [-0x0005]
// [-0x5]
