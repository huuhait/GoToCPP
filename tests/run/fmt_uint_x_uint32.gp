// phase: 05

// Go's %x of uint32 values with flags, width and precision.
package main

import "fmt"

func main() {
	fmt.Printf("[%x]\n", uint32(4294967295))
	fmt.Printf("[%+x]\n", uint32(4294967295))
	fmt.Printf("[%-x]\n", uint32(4294967295))
	fmt.Printf("[% x]\n", uint32(4294967295))
	fmt.Printf("[%0x]\n", uint32(4294967295))
	fmt.Printf("[%#x]\n", uint32(4294967295))
	fmt.Printf("[%8x]\n", uint32(4294967295))
	fmt.Printf("[%-8x]\n", uint32(4294967295))
	fmt.Printf("[%08x]\n", uint32(4294967295))
	fmt.Printf("[%+8x]\n", uint32(4294967295))
	fmt.Printf("[%+08x]\n", uint32(4294967295))
	fmt.Printf("[% 8x]\n", uint32(4294967295))
	fmt.Printf("[%8.3x]\n", uint32(4294967295))
	fmt.Printf("[%.3x]\n", uint32(4294967295))
	fmt.Printf("[%.0x]\n", uint32(4294967295))
	fmt.Printf("[%#8x]\n", uint32(4294967295))
	fmt.Printf("[%#08x]\n", uint32(4294967295))
	fmt.Printf("[%-#8x]\n", uint32(4294967295))
	fmt.Printf("[%+.5x]\n", uint32(4294967295))
	fmt.Printf("[%0-8x]\n", uint32(4294967295))
	fmt.Printf("[%12.8x]\n", uint32(4294967295))
	fmt.Printf("[%-12.8x]\n", uint32(4294967295))
	fmt.Printf("[%#.4x]\n", uint32(4294967295))
	fmt.Printf("[%+#x]\n", uint32(4294967295))
	fmt.Printf("[%x]\n", uint32(65))
	fmt.Printf("[%+x]\n", uint32(65))
	fmt.Printf("[%-x]\n", uint32(65))
	fmt.Printf("[% x]\n", uint32(65))
	fmt.Printf("[%0x]\n", uint32(65))
	fmt.Printf("[%#x]\n", uint32(65))
	fmt.Printf("[%8x]\n", uint32(65))
	fmt.Printf("[%-8x]\n", uint32(65))
	fmt.Printf("[%08x]\n", uint32(65))
	fmt.Printf("[%+8x]\n", uint32(65))
	fmt.Printf("[%+08x]\n", uint32(65))
	fmt.Printf("[% 8x]\n", uint32(65))
	fmt.Printf("[%8.3x]\n", uint32(65))
	fmt.Printf("[%.3x]\n", uint32(65))
	fmt.Printf("[%.0x]\n", uint32(65))
	fmt.Printf("[%#8x]\n", uint32(65))
	fmt.Printf("[%#08x]\n", uint32(65))
	fmt.Printf("[%-#8x]\n", uint32(65))
	fmt.Printf("[%+.5x]\n", uint32(65))
	fmt.Printf("[%0-8x]\n", uint32(65))
	fmt.Printf("[%12.8x]\n", uint32(65))
	fmt.Printf("[%-12.8x]\n", uint32(65))
	fmt.Printf("[%#.4x]\n", uint32(65))
	fmt.Printf("[%+#x]\n", uint32(65))
}

// Output:
// [ffffffff]
// [+ffffffff]
// [ffffffff]
// [ ffffffff]
// [ffffffff]
// [0xffffffff]
// [ffffffff]
// [ffffffff]
// [ffffffff]
// [+ffffffff]
// [+ffffffff]
// [ ffffffff]
// [ffffffff]
// [ffffffff]
// [ffffffff]
// [0xffffffff]
// [0xffffffff]
// [0xffffffff]
// [+ffffffff]
// [ffffffff]
// [    ffffffff]
// [ffffffff    ]
// [0xffffffff]
// [+0xffffffff]
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
