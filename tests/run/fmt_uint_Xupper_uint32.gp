// phase: 05

// Go's %X of uint32 values with flags, width and precision.
package main

import "fmt"

func main() {
	fmt.Printf("[%X]\n", uint32(4294967295))
	fmt.Printf("[%+X]\n", uint32(4294967295))
	fmt.Printf("[%-X]\n", uint32(4294967295))
	fmt.Printf("[% X]\n", uint32(4294967295))
	fmt.Printf("[%0X]\n", uint32(4294967295))
	fmt.Printf("[%#X]\n", uint32(4294967295))
	fmt.Printf("[%8X]\n", uint32(4294967295))
	fmt.Printf("[%-8X]\n", uint32(4294967295))
	fmt.Printf("[%08X]\n", uint32(4294967295))
	fmt.Printf("[%+8X]\n", uint32(4294967295))
	fmt.Printf("[%+08X]\n", uint32(4294967295))
	fmt.Printf("[% 8X]\n", uint32(4294967295))
	fmt.Printf("[%8.3X]\n", uint32(4294967295))
	fmt.Printf("[%.3X]\n", uint32(4294967295))
	fmt.Printf("[%.0X]\n", uint32(4294967295))
	fmt.Printf("[%#8X]\n", uint32(4294967295))
	fmt.Printf("[%#08X]\n", uint32(4294967295))
	fmt.Printf("[%-#8X]\n", uint32(4294967295))
	fmt.Printf("[%+.5X]\n", uint32(4294967295))
	fmt.Printf("[%0-8X]\n", uint32(4294967295))
	fmt.Printf("[%12.8X]\n", uint32(4294967295))
	fmt.Printf("[%-12.8X]\n", uint32(4294967295))
	fmt.Printf("[%#.4X]\n", uint32(4294967295))
	fmt.Printf("[%+#X]\n", uint32(4294967295))
	fmt.Printf("[%X]\n", uint32(65))
	fmt.Printf("[%+X]\n", uint32(65))
	fmt.Printf("[%-X]\n", uint32(65))
	fmt.Printf("[% X]\n", uint32(65))
	fmt.Printf("[%0X]\n", uint32(65))
	fmt.Printf("[%#X]\n", uint32(65))
	fmt.Printf("[%8X]\n", uint32(65))
	fmt.Printf("[%-8X]\n", uint32(65))
	fmt.Printf("[%08X]\n", uint32(65))
	fmt.Printf("[%+8X]\n", uint32(65))
	fmt.Printf("[%+08X]\n", uint32(65))
	fmt.Printf("[% 8X]\n", uint32(65))
	fmt.Printf("[%8.3X]\n", uint32(65))
	fmt.Printf("[%.3X]\n", uint32(65))
	fmt.Printf("[%.0X]\n", uint32(65))
	fmt.Printf("[%#8X]\n", uint32(65))
	fmt.Printf("[%#08X]\n", uint32(65))
	fmt.Printf("[%-#8X]\n", uint32(65))
	fmt.Printf("[%+.5X]\n", uint32(65))
	fmt.Printf("[%0-8X]\n", uint32(65))
	fmt.Printf("[%12.8X]\n", uint32(65))
	fmt.Printf("[%-12.8X]\n", uint32(65))
	fmt.Printf("[%#.4X]\n", uint32(65))
	fmt.Printf("[%+#X]\n", uint32(65))
}

// Output:
// [FFFFFFFF]
// [+FFFFFFFF]
// [FFFFFFFF]
// [ FFFFFFFF]
// [FFFFFFFF]
// [0XFFFFFFFF]
// [FFFFFFFF]
// [FFFFFFFF]
// [FFFFFFFF]
// [+FFFFFFFF]
// [+FFFFFFFF]
// [ FFFFFFFF]
// [FFFFFFFF]
// [FFFFFFFF]
// [FFFFFFFF]
// [0XFFFFFFFF]
// [0XFFFFFFFF]
// [0XFFFFFFFF]
// [+FFFFFFFF]
// [FFFFFFFF]
// [    FFFFFFFF]
// [FFFFFFFF    ]
// [0XFFFFFFFF]
// [+0XFFFFFFFF]
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
