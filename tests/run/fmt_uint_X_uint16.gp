// phase: 05

// Go's %X of uint16 values with flags, width and precision.
package main

import "fmt"

func main() {
	fmt.Printf("[%X]\n", uint16(65535))
	fmt.Printf("[%+X]\n", uint16(65535))
	fmt.Printf("[%-X]\n", uint16(65535))
	fmt.Printf("[% X]\n", uint16(65535))
	fmt.Printf("[%0X]\n", uint16(65535))
	fmt.Printf("[%#X]\n", uint16(65535))
	fmt.Printf("[%8X]\n", uint16(65535))
	fmt.Printf("[%-8X]\n", uint16(65535))
	fmt.Printf("[%08X]\n", uint16(65535))
	fmt.Printf("[%+8X]\n", uint16(65535))
	fmt.Printf("[%+08X]\n", uint16(65535))
	fmt.Printf("[% 8X]\n", uint16(65535))
	fmt.Printf("[%8.3X]\n", uint16(65535))
	fmt.Printf("[%.3X]\n", uint16(65535))
	fmt.Printf("[%.0X]\n", uint16(65535))
	fmt.Printf("[%#8X]\n", uint16(65535))
	fmt.Printf("[%#08X]\n", uint16(65535))
	fmt.Printf("[%-#8X]\n", uint16(65535))
	fmt.Printf("[%+.5X]\n", uint16(65535))
	fmt.Printf("[%0-8X]\n", uint16(65535))
	fmt.Printf("[%12.8X]\n", uint16(65535))
	fmt.Printf("[%-12.8X]\n", uint16(65535))
	fmt.Printf("[%#.4X]\n", uint16(65535))
	fmt.Printf("[%+#X]\n", uint16(65535))
	fmt.Printf("[%X]\n", uint16(300))
	fmt.Printf("[%+X]\n", uint16(300))
	fmt.Printf("[%-X]\n", uint16(300))
	fmt.Printf("[% X]\n", uint16(300))
	fmt.Printf("[%0X]\n", uint16(300))
	fmt.Printf("[%#X]\n", uint16(300))
	fmt.Printf("[%8X]\n", uint16(300))
	fmt.Printf("[%-8X]\n", uint16(300))
	fmt.Printf("[%08X]\n", uint16(300))
	fmt.Printf("[%+8X]\n", uint16(300))
	fmt.Printf("[%+08X]\n", uint16(300))
	fmt.Printf("[% 8X]\n", uint16(300))
	fmt.Printf("[%8.3X]\n", uint16(300))
	fmt.Printf("[%.3X]\n", uint16(300))
	fmt.Printf("[%.0X]\n", uint16(300))
	fmt.Printf("[%#8X]\n", uint16(300))
	fmt.Printf("[%#08X]\n", uint16(300))
	fmt.Printf("[%-#8X]\n", uint16(300))
	fmt.Printf("[%+.5X]\n", uint16(300))
	fmt.Printf("[%0-8X]\n", uint16(300))
	fmt.Printf("[%12.8X]\n", uint16(300))
	fmt.Printf("[%-12.8X]\n", uint16(300))
	fmt.Printf("[%#.4X]\n", uint16(300))
	fmt.Printf("[%+#X]\n", uint16(300))
}

// Output:
// [FFFF]
// [+FFFF]
// [FFFF]
// [ FFFF]
// [FFFF]
// [0XFFFF]
// [    FFFF]
// [FFFF    ]
// [0000FFFF]
// [   +FFFF]
// [+000FFFF]
// [    FFFF]
// [    FFFF]
// [FFFF]
// [FFFF]
// [  0XFFFF]
// [0X0000FFFF]
// [0XFFFF  ]
// [+0FFFF]
// [FFFF    ]
// [    0000FFFF]
// [0000FFFF    ]
// [0XFFFF]
// [+0XFFFF]
// [12C]
// [+12C]
// [12C]
// [ 12C]
// [12C]
// [0X12C]
// [     12C]
// [12C     ]
// [0000012C]
// [    +12C]
// [+000012C]
// [     12C]
// [     12C]
// [12C]
// [12C]
// [   0X12C]
// [0X0000012C]
// [0X12C   ]
// [+0012C]
// [12C     ]
// [    0000012C]
// [0000012C    ]
// [0X012C]
// [+0X12C]
