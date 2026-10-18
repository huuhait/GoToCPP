// phase: 05

// Go's %d of int8 values with every flag, width and precision: the integer formatting of
// fmt/format.go.
package main

import "fmt"

func main() {
	fmt.Printf("[%d]\n", int8(0))
	fmt.Printf("[%+d]\n", int8(0))
	fmt.Printf("[%-d]\n", int8(0))
	fmt.Printf("[% d]\n", int8(0))
	fmt.Printf("[%0d]\n", int8(0))
	fmt.Printf("[%#d]\n", int8(0))
	fmt.Printf("[%8d]\n", int8(0))
	fmt.Printf("[%-8d]\n", int8(0))
	fmt.Printf("[%08d]\n", int8(0))
	fmt.Printf("[%+8d]\n", int8(0))
	fmt.Printf("[%+08d]\n", int8(0))
	fmt.Printf("[% 8d]\n", int8(0))
	fmt.Printf("[%8.3d]\n", int8(0))
	fmt.Printf("[%.3d]\n", int8(0))
	fmt.Printf("[%.0d]\n", int8(0))
	fmt.Printf("[%#8d]\n", int8(0))
	fmt.Printf("[%#08d]\n", int8(0))
	fmt.Printf("[%-#8d]\n", int8(0))
	fmt.Printf("[%+.5d]\n", int8(0))
	fmt.Printf("[%0-8d]\n", int8(0))
	fmt.Printf("[%12.8d]\n", int8(0))
	fmt.Printf("[%-12.8d]\n", int8(0))
	fmt.Printf("[%#.4d]\n", int8(0))
	fmt.Printf("[%+#d]\n", int8(0))
	fmt.Printf("[%d]\n", int8(127))
	fmt.Printf("[%+d]\n", int8(127))
	fmt.Printf("[%-d]\n", int8(127))
	fmt.Printf("[% d]\n", int8(127))
	fmt.Printf("[%0d]\n", int8(127))
	fmt.Printf("[%#d]\n", int8(127))
	fmt.Printf("[%8d]\n", int8(127))
	fmt.Printf("[%-8d]\n", int8(127))
	fmt.Printf("[%08d]\n", int8(127))
	fmt.Printf("[%+8d]\n", int8(127))
	fmt.Printf("[%+08d]\n", int8(127))
	fmt.Printf("[% 8d]\n", int8(127))
	fmt.Printf("[%8.3d]\n", int8(127))
	fmt.Printf("[%.3d]\n", int8(127))
	fmt.Printf("[%.0d]\n", int8(127))
	fmt.Printf("[%#8d]\n", int8(127))
	fmt.Printf("[%#08d]\n", int8(127))
	fmt.Printf("[%-#8d]\n", int8(127))
	fmt.Printf("[%+.5d]\n", int8(127))
	fmt.Printf("[%0-8d]\n", int8(127))
	fmt.Printf("[%12.8d]\n", int8(127))
	fmt.Printf("[%-12.8d]\n", int8(127))
	fmt.Printf("[%#.4d]\n", int8(127))
	fmt.Printf("[%+#d]\n", int8(127))
	fmt.Printf("[%d]\n", int8(-128))
	fmt.Printf("[%+d]\n", int8(-128))
	fmt.Printf("[%-d]\n", int8(-128))
	fmt.Printf("[% d]\n", int8(-128))
	fmt.Printf("[%0d]\n", int8(-128))
	fmt.Printf("[%#d]\n", int8(-128))
	fmt.Printf("[%8d]\n", int8(-128))
	fmt.Printf("[%-8d]\n", int8(-128))
	fmt.Printf("[%08d]\n", int8(-128))
	fmt.Printf("[%+8d]\n", int8(-128))
	fmt.Printf("[%+08d]\n", int8(-128))
	fmt.Printf("[% 8d]\n", int8(-128))
	fmt.Printf("[%8.3d]\n", int8(-128))
	fmt.Printf("[%.3d]\n", int8(-128))
	fmt.Printf("[%.0d]\n", int8(-128))
	fmt.Printf("[%#8d]\n", int8(-128))
	fmt.Printf("[%#08d]\n", int8(-128))
	fmt.Printf("[%-#8d]\n", int8(-128))
	fmt.Printf("[%+.5d]\n", int8(-128))
	fmt.Printf("[%0-8d]\n", int8(-128))
	fmt.Printf("[%12.8d]\n", int8(-128))
	fmt.Printf("[%-12.8d]\n", int8(-128))
	fmt.Printf("[%#.4d]\n", int8(-128))
	fmt.Printf("[%+#d]\n", int8(-128))
	fmt.Printf("[%d]\n", int8(-5))
	fmt.Printf("[%+d]\n", int8(-5))
	fmt.Printf("[%-d]\n", int8(-5))
	fmt.Printf("[% d]\n", int8(-5))
	fmt.Printf("[%0d]\n", int8(-5))
	fmt.Printf("[%#d]\n", int8(-5))
	fmt.Printf("[%8d]\n", int8(-5))
	fmt.Printf("[%-8d]\n", int8(-5))
	fmt.Printf("[%08d]\n", int8(-5))
	fmt.Printf("[%+8d]\n", int8(-5))
	fmt.Printf("[%+08d]\n", int8(-5))
	fmt.Printf("[% 8d]\n", int8(-5))
	fmt.Printf("[%8.3d]\n", int8(-5))
	fmt.Printf("[%.3d]\n", int8(-5))
	fmt.Printf("[%.0d]\n", int8(-5))
	fmt.Printf("[%#8d]\n", int8(-5))
	fmt.Printf("[%#08d]\n", int8(-5))
	fmt.Printf("[%-#8d]\n", int8(-5))
	fmt.Printf("[%+.5d]\n", int8(-5))
	fmt.Printf("[%0-8d]\n", int8(-5))
	fmt.Printf("[%12.8d]\n", int8(-5))
	fmt.Printf("[%-12.8d]\n", int8(-5))
	fmt.Printf("[%#.4d]\n", int8(-5))
	fmt.Printf("[%+#d]\n", int8(-5))
}

// Output:
// [0]
// [+0]
// [0]
// [ 0]
// [0]
// [0]
// [       0]
// [0       ]
// [00000000]
// [      +0]
// [+0000000]
// [       0]
// [     000]
// [000]
// []
// [       0]
// [00000000]
// [0       ]
// [+00000]
// [0       ]
// [    00000000]
// [00000000    ]
// [0000]
// [+0]
// [127]
// [+127]
// [127]
// [ 127]
// [127]
// [127]
// [     127]
// [127     ]
// [00000127]
// [    +127]
// [+0000127]
// [     127]
// [     127]
// [127]
// [127]
// [     127]
// [00000127]
// [127     ]
// [+00127]
// [127     ]
// [    00000127]
// [00000127    ]
// [0127]
// [+127]
// [-128]
// [-128]
// [-128]
// [-128]
// [-128]
// [-128]
// [    -128]
// [-128    ]
// [-0000128]
// [    -128]
// [-0000128]
// [    -128]
// [    -128]
// [-128]
// [-128]
// [    -128]
// [-0000128]
// [-128    ]
// [-00128]
// [-128    ]
// [   -00000128]
// [-00000128   ]
// [-0128]
// [-128]
// [-5]
// [-5]
// [-5]
// [-5]
// [-5]
// [-5]
// [      -5]
// [-5      ]
// [-0000005]
// [      -5]
// [-0000005]
// [      -5]
// [    -005]
// [-005]
// [-5]
// [      -5]
// [-0000005]
// [-5      ]
// [-00005]
// [-5      ]
// [   -00000005]
// [-00000005   ]
// [-0005]
// [-5]
