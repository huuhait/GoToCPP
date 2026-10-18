// phase: 05

// Go's %d of uint8 values with flags, width and precision.
package main

import "fmt"

func main() {
	fmt.Printf("[%d]\n", uint8(0))
	fmt.Printf("[%+d]\n", uint8(0))
	fmt.Printf("[%-d]\n", uint8(0))
	fmt.Printf("[% d]\n", uint8(0))
	fmt.Printf("[%0d]\n", uint8(0))
	fmt.Printf("[%#d]\n", uint8(0))
	fmt.Printf("[%8d]\n", uint8(0))
	fmt.Printf("[%-8d]\n", uint8(0))
	fmt.Printf("[%08d]\n", uint8(0))
	fmt.Printf("[%+8d]\n", uint8(0))
	fmt.Printf("[%+08d]\n", uint8(0))
	fmt.Printf("[% 8d]\n", uint8(0))
	fmt.Printf("[%8.3d]\n", uint8(0))
	fmt.Printf("[%.3d]\n", uint8(0))
	fmt.Printf("[%.0d]\n", uint8(0))
	fmt.Printf("[%#8d]\n", uint8(0))
	fmt.Printf("[%#08d]\n", uint8(0))
	fmt.Printf("[%-#8d]\n", uint8(0))
	fmt.Printf("[%+.5d]\n", uint8(0))
	fmt.Printf("[%0-8d]\n", uint8(0))
	fmt.Printf("[%12.8d]\n", uint8(0))
	fmt.Printf("[%-12.8d]\n", uint8(0))
	fmt.Printf("[%#.4d]\n", uint8(0))
	fmt.Printf("[%+#d]\n", uint8(0))
	fmt.Printf("[%d]\n", uint8(255))
	fmt.Printf("[%+d]\n", uint8(255))
	fmt.Printf("[%-d]\n", uint8(255))
	fmt.Printf("[% d]\n", uint8(255))
	fmt.Printf("[%0d]\n", uint8(255))
	fmt.Printf("[%#d]\n", uint8(255))
	fmt.Printf("[%8d]\n", uint8(255))
	fmt.Printf("[%-8d]\n", uint8(255))
	fmt.Printf("[%08d]\n", uint8(255))
	fmt.Printf("[%+8d]\n", uint8(255))
	fmt.Printf("[%+08d]\n", uint8(255))
	fmt.Printf("[% 8d]\n", uint8(255))
	fmt.Printf("[%8.3d]\n", uint8(255))
	fmt.Printf("[%.3d]\n", uint8(255))
	fmt.Printf("[%.0d]\n", uint8(255))
	fmt.Printf("[%#8d]\n", uint8(255))
	fmt.Printf("[%#08d]\n", uint8(255))
	fmt.Printf("[%-#8d]\n", uint8(255))
	fmt.Printf("[%+.5d]\n", uint8(255))
	fmt.Printf("[%0-8d]\n", uint8(255))
	fmt.Printf("[%12.8d]\n", uint8(255))
	fmt.Printf("[%-12.8d]\n", uint8(255))
	fmt.Printf("[%#.4d]\n", uint8(255))
	fmt.Printf("[%+#d]\n", uint8(255))
	fmt.Printf("[%d]\n", uint8(65))
	fmt.Printf("[%+d]\n", uint8(65))
	fmt.Printf("[%-d]\n", uint8(65))
	fmt.Printf("[% d]\n", uint8(65))
	fmt.Printf("[%0d]\n", uint8(65))
	fmt.Printf("[%#d]\n", uint8(65))
	fmt.Printf("[%8d]\n", uint8(65))
	fmt.Printf("[%-8d]\n", uint8(65))
	fmt.Printf("[%08d]\n", uint8(65))
	fmt.Printf("[%+8d]\n", uint8(65))
	fmt.Printf("[%+08d]\n", uint8(65))
	fmt.Printf("[% 8d]\n", uint8(65))
	fmt.Printf("[%8.3d]\n", uint8(65))
	fmt.Printf("[%.3d]\n", uint8(65))
	fmt.Printf("[%.0d]\n", uint8(65))
	fmt.Printf("[%#8d]\n", uint8(65))
	fmt.Printf("[%#08d]\n", uint8(65))
	fmt.Printf("[%-#8d]\n", uint8(65))
	fmt.Printf("[%+.5d]\n", uint8(65))
	fmt.Printf("[%0-8d]\n", uint8(65))
	fmt.Printf("[%12.8d]\n", uint8(65))
	fmt.Printf("[%-12.8d]\n", uint8(65))
	fmt.Printf("[%#.4d]\n", uint8(65))
	fmt.Printf("[%+#d]\n", uint8(65))
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
// [255]
// [+255]
// [255]
// [ 255]
// [255]
// [255]
// [     255]
// [255     ]
// [00000255]
// [    +255]
// [+0000255]
// [     255]
// [     255]
// [255]
// [255]
// [     255]
// [00000255]
// [255     ]
// [+00255]
// [255     ]
// [    00000255]
// [00000255    ]
// [0255]
// [+255]
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
