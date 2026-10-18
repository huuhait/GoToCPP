// phase: 05

// Go's %x of uintptr values with flags, width and precision.
package main

import "fmt"

func main() {
	fmt.Printf("[%x]\n", uintptr(0))
	fmt.Printf("[%+x]\n", uintptr(0))
	fmt.Printf("[%-x]\n", uintptr(0))
	fmt.Printf("[% x]\n", uintptr(0))
	fmt.Printf("[%0x]\n", uintptr(0))
	fmt.Printf("[%#x]\n", uintptr(0))
	fmt.Printf("[%8x]\n", uintptr(0))
	fmt.Printf("[%-8x]\n", uintptr(0))
	fmt.Printf("[%08x]\n", uintptr(0))
	fmt.Printf("[%+8x]\n", uintptr(0))
	fmt.Printf("[%+08x]\n", uintptr(0))
	fmt.Printf("[% 8x]\n", uintptr(0))
	fmt.Printf("[%8.3x]\n", uintptr(0))
	fmt.Printf("[%.3x]\n", uintptr(0))
	fmt.Printf("[%.0x]\n", uintptr(0))
	fmt.Printf("[%#8x]\n", uintptr(0))
	fmt.Printf("[%#08x]\n", uintptr(0))
	fmt.Printf("[%-#8x]\n", uintptr(0))
	fmt.Printf("[%+.5x]\n", uintptr(0))
	fmt.Printf("[%0-8x]\n", uintptr(0))
	fmt.Printf("[%12.8x]\n", uintptr(0))
	fmt.Printf("[%-12.8x]\n", uintptr(0))
	fmt.Printf("[%#.4x]\n", uintptr(0))
	fmt.Printf("[%+#x]\n", uintptr(0))
	fmt.Printf("[%x]\n", uintptr(4096))
	fmt.Printf("[%+x]\n", uintptr(4096))
	fmt.Printf("[%-x]\n", uintptr(4096))
	fmt.Printf("[% x]\n", uintptr(4096))
	fmt.Printf("[%0x]\n", uintptr(4096))
	fmt.Printf("[%#x]\n", uintptr(4096))
	fmt.Printf("[%8x]\n", uintptr(4096))
	fmt.Printf("[%-8x]\n", uintptr(4096))
	fmt.Printf("[%08x]\n", uintptr(4096))
	fmt.Printf("[%+8x]\n", uintptr(4096))
	fmt.Printf("[%+08x]\n", uintptr(4096))
	fmt.Printf("[% 8x]\n", uintptr(4096))
	fmt.Printf("[%8.3x]\n", uintptr(4096))
	fmt.Printf("[%.3x]\n", uintptr(4096))
	fmt.Printf("[%.0x]\n", uintptr(4096))
	fmt.Printf("[%#8x]\n", uintptr(4096))
	fmt.Printf("[%#08x]\n", uintptr(4096))
	fmt.Printf("[%-#8x]\n", uintptr(4096))
	fmt.Printf("[%+.5x]\n", uintptr(4096))
	fmt.Printf("[%0-8x]\n", uintptr(4096))
	fmt.Printf("[%12.8x]\n", uintptr(4096))
	fmt.Printf("[%-12.8x]\n", uintptr(4096))
	fmt.Printf("[%#.4x]\n", uintptr(4096))
	fmt.Printf("[%+#x]\n", uintptr(4096))
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
// [1000]
// [+1000]
// [1000]
// [ 1000]
// [1000]
// [0x1000]
// [    1000]
// [1000    ]
// [00001000]
// [   +1000]
// [+0001000]
// [    1000]
// [    1000]
// [1000]
// [1000]
// [  0x1000]
// [0x00001000]
// [0x1000  ]
// [+01000]
// [1000    ]
// [    00001000]
// [00001000    ]
// [0x1000]
// [+0x1000]
