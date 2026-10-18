// phase: 05

// Go's %x of int values with every flag, width and precision: the integer formatting of
// fmt/format.go.
package main

import "fmt"

func main() {
	fmt.Printf("[%x]\n", 0)
	fmt.Printf("[%+x]\n", 0)
	fmt.Printf("[%-x]\n", 0)
	fmt.Printf("[% x]\n", 0)
	fmt.Printf("[%0x]\n", 0)
	fmt.Printf("[%#x]\n", 0)
	fmt.Printf("[%8x]\n", 0)
	fmt.Printf("[%-8x]\n", 0)
	fmt.Printf("[%08x]\n", 0)
	fmt.Printf("[%+8x]\n", 0)
	fmt.Printf("[%+08x]\n", 0)
	fmt.Printf("[% 8x]\n", 0)
	fmt.Printf("[%8.3x]\n", 0)
	fmt.Printf("[%.3x]\n", 0)
	fmt.Printf("[%.0x]\n", 0)
	fmt.Printf("[%#8x]\n", 0)
	fmt.Printf("[%#08x]\n", 0)
	fmt.Printf("[%-#8x]\n", 0)
	fmt.Printf("[%+.5x]\n", 0)
	fmt.Printf("[%0-8x]\n", 0)
	fmt.Printf("[%12.8x]\n", 0)
	fmt.Printf("[%-12.8x]\n", 0)
	fmt.Printf("[%#.4x]\n", 0)
	fmt.Printf("[%+#x]\n", 0)
	fmt.Printf("[%x]\n", 1)
	fmt.Printf("[%+x]\n", 1)
	fmt.Printf("[%-x]\n", 1)
	fmt.Printf("[% x]\n", 1)
	fmt.Printf("[%0x]\n", 1)
	fmt.Printf("[%#x]\n", 1)
	fmt.Printf("[%8x]\n", 1)
	fmt.Printf("[%-8x]\n", 1)
	fmt.Printf("[%08x]\n", 1)
	fmt.Printf("[%+8x]\n", 1)
	fmt.Printf("[%+08x]\n", 1)
	fmt.Printf("[% 8x]\n", 1)
	fmt.Printf("[%8.3x]\n", 1)
	fmt.Printf("[%.3x]\n", 1)
	fmt.Printf("[%.0x]\n", 1)
	fmt.Printf("[%#8x]\n", 1)
	fmt.Printf("[%#08x]\n", 1)
	fmt.Printf("[%-#8x]\n", 1)
	fmt.Printf("[%+.5x]\n", 1)
	fmt.Printf("[%0-8x]\n", 1)
	fmt.Printf("[%12.8x]\n", 1)
	fmt.Printf("[%-12.8x]\n", 1)
	fmt.Printf("[%#.4x]\n", 1)
	fmt.Printf("[%+#x]\n", 1)
	fmt.Printf("[%x]\n", -1)
	fmt.Printf("[%+x]\n", -1)
	fmt.Printf("[%-x]\n", -1)
	fmt.Printf("[% x]\n", -1)
	fmt.Printf("[%0x]\n", -1)
	fmt.Printf("[%#x]\n", -1)
	fmt.Printf("[%8x]\n", -1)
	fmt.Printf("[%-8x]\n", -1)
	fmt.Printf("[%08x]\n", -1)
	fmt.Printf("[%+8x]\n", -1)
	fmt.Printf("[%+08x]\n", -1)
	fmt.Printf("[% 8x]\n", -1)
	fmt.Printf("[%8.3x]\n", -1)
	fmt.Printf("[%.3x]\n", -1)
	fmt.Printf("[%.0x]\n", -1)
	fmt.Printf("[%#8x]\n", -1)
	fmt.Printf("[%#08x]\n", -1)
	fmt.Printf("[%-#8x]\n", -1)
	fmt.Printf("[%+.5x]\n", -1)
	fmt.Printf("[%0-8x]\n", -1)
	fmt.Printf("[%12.8x]\n", -1)
	fmt.Printf("[%-12.8x]\n", -1)
	fmt.Printf("[%#.4x]\n", -1)
	fmt.Printf("[%+#x]\n", -1)
	fmt.Printf("[%x]\n", 42)
	fmt.Printf("[%+x]\n", 42)
	fmt.Printf("[%-x]\n", 42)
	fmt.Printf("[% x]\n", 42)
	fmt.Printf("[%0x]\n", 42)
	fmt.Printf("[%#x]\n", 42)
	fmt.Printf("[%8x]\n", 42)
	fmt.Printf("[%-8x]\n", 42)
	fmt.Printf("[%08x]\n", 42)
	fmt.Printf("[%+8x]\n", 42)
	fmt.Printf("[%+08x]\n", 42)
	fmt.Printf("[% 8x]\n", 42)
	fmt.Printf("[%8.3x]\n", 42)
	fmt.Printf("[%.3x]\n", 42)
	fmt.Printf("[%.0x]\n", 42)
	fmt.Printf("[%#8x]\n", 42)
	fmt.Printf("[%#08x]\n", 42)
	fmt.Printf("[%-#8x]\n", 42)
	fmt.Printf("[%+.5x]\n", 42)
	fmt.Printf("[%0-8x]\n", 42)
	fmt.Printf("[%12.8x]\n", 42)
	fmt.Printf("[%-12.8x]\n", 42)
	fmt.Printf("[%#.4x]\n", 42)
	fmt.Printf("[%+#x]\n", 42)
	fmt.Printf("[%x]\n", -42)
	fmt.Printf("[%+x]\n", -42)
	fmt.Printf("[%-x]\n", -42)
	fmt.Printf("[% x]\n", -42)
	fmt.Printf("[%0x]\n", -42)
	fmt.Printf("[%#x]\n", -42)
	fmt.Printf("[%8x]\n", -42)
	fmt.Printf("[%-8x]\n", -42)
	fmt.Printf("[%08x]\n", -42)
	fmt.Printf("[%+8x]\n", -42)
	fmt.Printf("[%+08x]\n", -42)
	fmt.Printf("[% 8x]\n", -42)
	fmt.Printf("[%8.3x]\n", -42)
	fmt.Printf("[%.3x]\n", -42)
	fmt.Printf("[%.0x]\n", -42)
	fmt.Printf("[%#8x]\n", -42)
	fmt.Printf("[%#08x]\n", -42)
	fmt.Printf("[%-#8x]\n", -42)
	fmt.Printf("[%+.5x]\n", -42)
	fmt.Printf("[%0-8x]\n", -42)
	fmt.Printf("[%12.8x]\n", -42)
	fmt.Printf("[%-12.8x]\n", -42)
	fmt.Printf("[%#.4x]\n", -42)
	fmt.Printf("[%+#x]\n", -42)
	fmt.Printf("[%x]\n", 1234567)
	fmt.Printf("[%+x]\n", 1234567)
	fmt.Printf("[%-x]\n", 1234567)
	fmt.Printf("[% x]\n", 1234567)
	fmt.Printf("[%0x]\n", 1234567)
	fmt.Printf("[%#x]\n", 1234567)
	fmt.Printf("[%8x]\n", 1234567)
	fmt.Printf("[%-8x]\n", 1234567)
	fmt.Printf("[%08x]\n", 1234567)
	fmt.Printf("[%+8x]\n", 1234567)
	fmt.Printf("[%+08x]\n", 1234567)
	fmt.Printf("[% 8x]\n", 1234567)
	fmt.Printf("[%8.3x]\n", 1234567)
	fmt.Printf("[%.3x]\n", 1234567)
	fmt.Printf("[%.0x]\n", 1234567)
	fmt.Printf("[%#8x]\n", 1234567)
	fmt.Printf("[%#08x]\n", 1234567)
	fmt.Printf("[%-#8x]\n", 1234567)
	fmt.Printf("[%+.5x]\n", 1234567)
	fmt.Printf("[%0-8x]\n", 1234567)
	fmt.Printf("[%12.8x]\n", 1234567)
	fmt.Printf("[%-12.8x]\n", 1234567)
	fmt.Printf("[%#.4x]\n", 1234567)
	fmt.Printf("[%+#x]\n", 1234567)
	fmt.Printf("[%x]\n", -1234567)
	fmt.Printf("[%+x]\n", -1234567)
	fmt.Printf("[%-x]\n", -1234567)
	fmt.Printf("[% x]\n", -1234567)
	fmt.Printf("[%0x]\n", -1234567)
	fmt.Printf("[%#x]\n", -1234567)
	fmt.Printf("[%8x]\n", -1234567)
	fmt.Printf("[%-8x]\n", -1234567)
	fmt.Printf("[%08x]\n", -1234567)
	fmt.Printf("[%+8x]\n", -1234567)
	fmt.Printf("[%+08x]\n", -1234567)
	fmt.Printf("[% 8x]\n", -1234567)
	fmt.Printf("[%8.3x]\n", -1234567)
	fmt.Printf("[%.3x]\n", -1234567)
	fmt.Printf("[%.0x]\n", -1234567)
	fmt.Printf("[%#8x]\n", -1234567)
	fmt.Printf("[%#08x]\n", -1234567)
	fmt.Printf("[%-#8x]\n", -1234567)
	fmt.Printf("[%+.5x]\n", -1234567)
	fmt.Printf("[%0-8x]\n", -1234567)
	fmt.Printf("[%12.8x]\n", -1234567)
	fmt.Printf("[%-12.8x]\n", -1234567)
	fmt.Printf("[%#.4x]\n", -1234567)
	fmt.Printf("[%+#x]\n", -1234567)
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
// [1]
// [+1]
// [1]
// [ 1]
// [1]
// [0x1]
// [       1]
// [1       ]
// [00000001]
// [      +1]
// [+0000001]
// [       1]
// [     001]
// [001]
// [1]
// [     0x1]
// [0x00000001]
// [0x1     ]
// [+00001]
// [1       ]
// [    00000001]
// [00000001    ]
// [0x0001]
// [+0x1]
// [-1]
// [-1]
// [-1]
// [-1]
// [-1]
// [-0x1]
// [      -1]
// [-1      ]
// [-0000001]
// [      -1]
// [-0000001]
// [      -1]
// [    -001]
// [-001]
// [-1]
// [    -0x1]
// [-0x0000001]
// [-0x1    ]
// [-00001]
// [-1      ]
// [   -00000001]
// [-00000001   ]
// [-0x0001]
// [-0x1]
// [2a]
// [+2a]
// [2a]
// [ 2a]
// [2a]
// [0x2a]
// [      2a]
// [2a      ]
// [0000002a]
// [     +2a]
// [+000002a]
// [      2a]
// [     02a]
// [02a]
// [2a]
// [    0x2a]
// [0x0000002a]
// [0x2a    ]
// [+0002a]
// [2a      ]
// [    0000002a]
// [0000002a    ]
// [0x002a]
// [+0x2a]
// [-2a]
// [-2a]
// [-2a]
// [-2a]
// [-2a]
// [-0x2a]
// [     -2a]
// [-2a     ]
// [-000002a]
// [     -2a]
// [-000002a]
// [     -2a]
// [    -02a]
// [-02a]
// [-2a]
// [   -0x2a]
// [-0x000002a]
// [-0x2a   ]
// [-0002a]
// [-2a     ]
// [   -0000002a]
// [-0000002a   ]
// [-0x002a]
// [-0x2a]
// [12d687]
// [+12d687]
// [12d687]
// [ 12d687]
// [12d687]
// [0x12d687]
// [  12d687]
// [12d687  ]
// [0012d687]
// [ +12d687]
// [+012d687]
// [  12d687]
// [  12d687]
// [12d687]
// [12d687]
// [0x12d687]
// [0x0012d687]
// [0x12d687]
// [+12d687]
// [12d687  ]
// [    0012d687]
// [0012d687    ]
// [0x12d687]
// [+0x12d687]
// [-12d687]
// [-12d687]
// [-12d687]
// [-12d687]
// [-12d687]
// [-0x12d687]
// [ -12d687]
// [-12d687 ]
// [-012d687]
// [ -12d687]
// [-012d687]
// [ -12d687]
// [ -12d687]
// [-12d687]
// [-12d687]
// [-0x12d687]
// [-0x012d687]
// [-0x12d687]
// [-12d687]
// [-12d687 ]
// [   -0012d687]
// [-0012d687   ]
// [-0x12d687]
// [-0x12d687]
