// phase: 05

// Go's %X of uint8 values with flags, width and precision.
package main

import "fmt"

func main() {
	fmt.Printf("[%X]\n", uint8(0))
	fmt.Printf("[%+X]\n", uint8(0))
	fmt.Printf("[%-X]\n", uint8(0))
	fmt.Printf("[% X]\n", uint8(0))
	fmt.Printf("[%0X]\n", uint8(0))
	fmt.Printf("[%#X]\n", uint8(0))
	fmt.Printf("[%8X]\n", uint8(0))
	fmt.Printf("[%-8X]\n", uint8(0))
	fmt.Printf("[%08X]\n", uint8(0))
	fmt.Printf("[%+8X]\n", uint8(0))
	fmt.Printf("[%+08X]\n", uint8(0))
	fmt.Printf("[% 8X]\n", uint8(0))
	fmt.Printf("[%8.3X]\n", uint8(0))
	fmt.Printf("[%.3X]\n", uint8(0))
	fmt.Printf("[%.0X]\n", uint8(0))
	fmt.Printf("[%#8X]\n", uint8(0))
	fmt.Printf("[%#08X]\n", uint8(0))
	fmt.Printf("[%-#8X]\n", uint8(0))
	fmt.Printf("[%+.5X]\n", uint8(0))
	fmt.Printf("[%0-8X]\n", uint8(0))
	fmt.Printf("[%12.8X]\n", uint8(0))
	fmt.Printf("[%-12.8X]\n", uint8(0))
	fmt.Printf("[%#.4X]\n", uint8(0))
	fmt.Printf("[%+#X]\n", uint8(0))
	fmt.Printf("[%X]\n", uint8(255))
	fmt.Printf("[%+X]\n", uint8(255))
	fmt.Printf("[%-X]\n", uint8(255))
	fmt.Printf("[% X]\n", uint8(255))
	fmt.Printf("[%0X]\n", uint8(255))
	fmt.Printf("[%#X]\n", uint8(255))
	fmt.Printf("[%8X]\n", uint8(255))
	fmt.Printf("[%-8X]\n", uint8(255))
	fmt.Printf("[%08X]\n", uint8(255))
	fmt.Printf("[%+8X]\n", uint8(255))
	fmt.Printf("[%+08X]\n", uint8(255))
	fmt.Printf("[% 8X]\n", uint8(255))
	fmt.Printf("[%8.3X]\n", uint8(255))
	fmt.Printf("[%.3X]\n", uint8(255))
	fmt.Printf("[%.0X]\n", uint8(255))
	fmt.Printf("[%#8X]\n", uint8(255))
	fmt.Printf("[%#08X]\n", uint8(255))
	fmt.Printf("[%-#8X]\n", uint8(255))
	fmt.Printf("[%+.5X]\n", uint8(255))
	fmt.Printf("[%0-8X]\n", uint8(255))
	fmt.Printf("[%12.8X]\n", uint8(255))
	fmt.Printf("[%-12.8X]\n", uint8(255))
	fmt.Printf("[%#.4X]\n", uint8(255))
	fmt.Printf("[%+#X]\n", uint8(255))
	fmt.Printf("[%X]\n", uint8(65))
	fmt.Printf("[%+X]\n", uint8(65))
	fmt.Printf("[%-X]\n", uint8(65))
	fmt.Printf("[% X]\n", uint8(65))
	fmt.Printf("[%0X]\n", uint8(65))
	fmt.Printf("[%#X]\n", uint8(65))
	fmt.Printf("[%8X]\n", uint8(65))
	fmt.Printf("[%-8X]\n", uint8(65))
	fmt.Printf("[%08X]\n", uint8(65))
	fmt.Printf("[%+8X]\n", uint8(65))
	fmt.Printf("[%+08X]\n", uint8(65))
	fmt.Printf("[% 8X]\n", uint8(65))
	fmt.Printf("[%8.3X]\n", uint8(65))
	fmt.Printf("[%.3X]\n", uint8(65))
	fmt.Printf("[%.0X]\n", uint8(65))
	fmt.Printf("[%#8X]\n", uint8(65))
	fmt.Printf("[%#08X]\n", uint8(65))
	fmt.Printf("[%-#8X]\n", uint8(65))
	fmt.Printf("[%+.5X]\n", uint8(65))
	fmt.Printf("[%0-8X]\n", uint8(65))
	fmt.Printf("[%12.8X]\n", uint8(65))
	fmt.Printf("[%-12.8X]\n", uint8(65))
	fmt.Printf("[%#.4X]\n", uint8(65))
	fmt.Printf("[%+#X]\n", uint8(65))
}

// Output:
// [0]
// [+0]
// [0]
// [ 0]
// [0]
// [0X0]
// [       0]
// [0       ]
// [00000000]
// [      +0]
// [+0000000]
// [       0]
// [     000]
// [000]
// []
// [     0X0]
// [0X00000000]
// [0X0     ]
// [+00000]
// [0       ]
// [    00000000]
// [00000000    ]
// [0X0000]
// [+0X0]
// [FF]
// [+FF]
// [FF]
// [ FF]
// [FF]
// [0XFF]
// [      FF]
// [FF      ]
// [000000FF]
// [     +FF]
// [+00000FF]
// [      FF]
// [     0FF]
// [0FF]
// [FF]
// [    0XFF]
// [0X000000FF]
// [0XFF    ]
// [+000FF]
// [FF      ]
// [    000000FF]
// [000000FF    ]
// [0X00FF]
// [+0XFF]
// [41]
// [+41]
// [41]
// [ 41]
// [41]
// [0X41]
// [      41]
// [41      ]
// [00000041]
// [     +41]
// [+0000041]
// [      41]
// [     041]
// [041]
// [41]
// [    0X41]
// [0X00000041]
// [0X41    ]
// [+00041]
// [41      ]
// [    00000041]
// [00000041    ]
// [0X0041]
// [+0X41]
