// phase: 05

// Go's %x of uint8 values with flags, width and precision.
package main

import "fmt"

func main() {
	fmt.Printf("[%x]\n", uint8(0))
	fmt.Printf("[%+x]\n", uint8(0))
	fmt.Printf("[%-x]\n", uint8(0))
	fmt.Printf("[% x]\n", uint8(0))
	fmt.Printf("[%0x]\n", uint8(0))
	fmt.Printf("[%#x]\n", uint8(0))
	fmt.Printf("[%8x]\n", uint8(0))
	fmt.Printf("[%-8x]\n", uint8(0))
	fmt.Printf("[%08x]\n", uint8(0))
	fmt.Printf("[%+8x]\n", uint8(0))
	fmt.Printf("[%+08x]\n", uint8(0))
	fmt.Printf("[% 8x]\n", uint8(0))
	fmt.Printf("[%8.3x]\n", uint8(0))
	fmt.Printf("[%.3x]\n", uint8(0))
	fmt.Printf("[%.0x]\n", uint8(0))
	fmt.Printf("[%#8x]\n", uint8(0))
	fmt.Printf("[%#08x]\n", uint8(0))
	fmt.Printf("[%-#8x]\n", uint8(0))
	fmt.Printf("[%+.5x]\n", uint8(0))
	fmt.Printf("[%0-8x]\n", uint8(0))
	fmt.Printf("[%12.8x]\n", uint8(0))
	fmt.Printf("[%-12.8x]\n", uint8(0))
	fmt.Printf("[%#.4x]\n", uint8(0))
	fmt.Printf("[%+#x]\n", uint8(0))
	fmt.Printf("[%x]\n", uint8(255))
	fmt.Printf("[%+x]\n", uint8(255))
	fmt.Printf("[%-x]\n", uint8(255))
	fmt.Printf("[% x]\n", uint8(255))
	fmt.Printf("[%0x]\n", uint8(255))
	fmt.Printf("[%#x]\n", uint8(255))
	fmt.Printf("[%8x]\n", uint8(255))
	fmt.Printf("[%-8x]\n", uint8(255))
	fmt.Printf("[%08x]\n", uint8(255))
	fmt.Printf("[%+8x]\n", uint8(255))
	fmt.Printf("[%+08x]\n", uint8(255))
	fmt.Printf("[% 8x]\n", uint8(255))
	fmt.Printf("[%8.3x]\n", uint8(255))
	fmt.Printf("[%.3x]\n", uint8(255))
	fmt.Printf("[%.0x]\n", uint8(255))
	fmt.Printf("[%#8x]\n", uint8(255))
	fmt.Printf("[%#08x]\n", uint8(255))
	fmt.Printf("[%-#8x]\n", uint8(255))
	fmt.Printf("[%+.5x]\n", uint8(255))
	fmt.Printf("[%0-8x]\n", uint8(255))
	fmt.Printf("[%12.8x]\n", uint8(255))
	fmt.Printf("[%-12.8x]\n", uint8(255))
	fmt.Printf("[%#.4x]\n", uint8(255))
	fmt.Printf("[%+#x]\n", uint8(255))
	fmt.Printf("[%x]\n", uint8(65))
	fmt.Printf("[%+x]\n", uint8(65))
	fmt.Printf("[%-x]\n", uint8(65))
	fmt.Printf("[% x]\n", uint8(65))
	fmt.Printf("[%0x]\n", uint8(65))
	fmt.Printf("[%#x]\n", uint8(65))
	fmt.Printf("[%8x]\n", uint8(65))
	fmt.Printf("[%-8x]\n", uint8(65))
	fmt.Printf("[%08x]\n", uint8(65))
	fmt.Printf("[%+8x]\n", uint8(65))
	fmt.Printf("[%+08x]\n", uint8(65))
	fmt.Printf("[% 8x]\n", uint8(65))
	fmt.Printf("[%8.3x]\n", uint8(65))
	fmt.Printf("[%.3x]\n", uint8(65))
	fmt.Printf("[%.0x]\n", uint8(65))
	fmt.Printf("[%#8x]\n", uint8(65))
	fmt.Printf("[%#08x]\n", uint8(65))
	fmt.Printf("[%-#8x]\n", uint8(65))
	fmt.Printf("[%+.5x]\n", uint8(65))
	fmt.Printf("[%0-8x]\n", uint8(65))
	fmt.Printf("[%12.8x]\n", uint8(65))
	fmt.Printf("[%-12.8x]\n", uint8(65))
	fmt.Printf("[%#.4x]\n", uint8(65))
	fmt.Printf("[%+#x]\n", uint8(65))
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
// [ff]
// [+ff]
// [ff]
// [ ff]
// [ff]
// [0xff]
// [      ff]
// [ff      ]
// [000000ff]
// [     +ff]
// [+00000ff]
// [      ff]
// [     0ff]
// [0ff]
// [ff]
// [    0xff]
// [0x000000ff]
// [0xff    ]
// [+000ff]
// [ff      ]
// [    000000ff]
// [000000ff    ]
// [0x00ff]
// [+0xff]
// [41]
// [+41]
// [41]
// [ 41]
// [41]
// [0x41]
// [      41]
// [41      ]
// [00000041]
// [     +41]
// [+0000041]
// [      41]
// [     041]
// [041]
// [41]
// [    0x41]
// [0x00000041]
// [0x41    ]
// [+00041]
// [41      ]
// [    00000041]
// [00000041    ]
// [0x0041]
// [+0x41]
