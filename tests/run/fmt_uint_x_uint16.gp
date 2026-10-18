// phase: 05

// Go's %x of uint16 values with flags, width and precision.
package main

import "fmt"

func main() {
	fmt.Printf("[%x]\n", uint16(65535))
	fmt.Printf("[%+x]\n", uint16(65535))
	fmt.Printf("[%-x]\n", uint16(65535))
	fmt.Printf("[% x]\n", uint16(65535))
	fmt.Printf("[%0x]\n", uint16(65535))
	fmt.Printf("[%#x]\n", uint16(65535))
	fmt.Printf("[%8x]\n", uint16(65535))
	fmt.Printf("[%-8x]\n", uint16(65535))
	fmt.Printf("[%08x]\n", uint16(65535))
	fmt.Printf("[%+8x]\n", uint16(65535))
	fmt.Printf("[%+08x]\n", uint16(65535))
	fmt.Printf("[% 8x]\n", uint16(65535))
	fmt.Printf("[%8.3x]\n", uint16(65535))
	fmt.Printf("[%.3x]\n", uint16(65535))
	fmt.Printf("[%.0x]\n", uint16(65535))
	fmt.Printf("[%#8x]\n", uint16(65535))
	fmt.Printf("[%#08x]\n", uint16(65535))
	fmt.Printf("[%-#8x]\n", uint16(65535))
	fmt.Printf("[%+.5x]\n", uint16(65535))
	fmt.Printf("[%0-8x]\n", uint16(65535))
	fmt.Printf("[%12.8x]\n", uint16(65535))
	fmt.Printf("[%-12.8x]\n", uint16(65535))
	fmt.Printf("[%#.4x]\n", uint16(65535))
	fmt.Printf("[%+#x]\n", uint16(65535))
	fmt.Printf("[%x]\n", uint16(300))
	fmt.Printf("[%+x]\n", uint16(300))
	fmt.Printf("[%-x]\n", uint16(300))
	fmt.Printf("[% x]\n", uint16(300))
	fmt.Printf("[%0x]\n", uint16(300))
	fmt.Printf("[%#x]\n", uint16(300))
	fmt.Printf("[%8x]\n", uint16(300))
	fmt.Printf("[%-8x]\n", uint16(300))
	fmt.Printf("[%08x]\n", uint16(300))
	fmt.Printf("[%+8x]\n", uint16(300))
	fmt.Printf("[%+08x]\n", uint16(300))
	fmt.Printf("[% 8x]\n", uint16(300))
	fmt.Printf("[%8.3x]\n", uint16(300))
	fmt.Printf("[%.3x]\n", uint16(300))
	fmt.Printf("[%.0x]\n", uint16(300))
	fmt.Printf("[%#8x]\n", uint16(300))
	fmt.Printf("[%#08x]\n", uint16(300))
	fmt.Printf("[%-#8x]\n", uint16(300))
	fmt.Printf("[%+.5x]\n", uint16(300))
	fmt.Printf("[%0-8x]\n", uint16(300))
	fmt.Printf("[%12.8x]\n", uint16(300))
	fmt.Printf("[%-12.8x]\n", uint16(300))
	fmt.Printf("[%#.4x]\n", uint16(300))
	fmt.Printf("[%+#x]\n", uint16(300))
}

// Output:
// [ffff]
// [+ffff]
// [ffff]
// [ ffff]
// [ffff]
// [0xffff]
// [    ffff]
// [ffff    ]
// [0000ffff]
// [   +ffff]
// [+000ffff]
// [    ffff]
// [    ffff]
// [ffff]
// [ffff]
// [  0xffff]
// [0x0000ffff]
// [0xffff  ]
// [+0ffff]
// [ffff    ]
// [    0000ffff]
// [0000ffff    ]
// [0xffff]
// [+0xffff]
// [12c]
// [+12c]
// [12c]
// [ 12c]
// [12c]
// [0x12c]
// [     12c]
// [12c     ]
// [0000012c]
// [    +12c]
// [+000012c]
// [     12c]
// [     12c]
// [12c]
// [12c]
// [   0x12c]
// [0x0000012c]
// [0x12c   ]
// [+0012c]
// [12c     ]
// [    0000012c]
// [0000012c    ]
// [0x012c]
// [+0x12c]
