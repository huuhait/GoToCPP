// phase: 05

// Go's %X of uintptr values with flags, width and precision.
package main

import "fmt"

func main() {
	fmt.Printf("[%X]\n", uintptr(0))
	fmt.Printf("[%+X]\n", uintptr(0))
	fmt.Printf("[%-X]\n", uintptr(0))
	fmt.Printf("[% X]\n", uintptr(0))
	fmt.Printf("[%0X]\n", uintptr(0))
	fmt.Printf("[%#X]\n", uintptr(0))
	fmt.Printf("[%8X]\n", uintptr(0))
	fmt.Printf("[%-8X]\n", uintptr(0))
	fmt.Printf("[%08X]\n", uintptr(0))
	fmt.Printf("[%+8X]\n", uintptr(0))
	fmt.Printf("[%+08X]\n", uintptr(0))
	fmt.Printf("[% 8X]\n", uintptr(0))
	fmt.Printf("[%8.3X]\n", uintptr(0))
	fmt.Printf("[%.3X]\n", uintptr(0))
	fmt.Printf("[%.0X]\n", uintptr(0))
	fmt.Printf("[%#8X]\n", uintptr(0))
	fmt.Printf("[%#08X]\n", uintptr(0))
	fmt.Printf("[%-#8X]\n", uintptr(0))
	fmt.Printf("[%+.5X]\n", uintptr(0))
	fmt.Printf("[%0-8X]\n", uintptr(0))
	fmt.Printf("[%12.8X]\n", uintptr(0))
	fmt.Printf("[%-12.8X]\n", uintptr(0))
	fmt.Printf("[%#.4X]\n", uintptr(0))
	fmt.Printf("[%+#X]\n", uintptr(0))
	fmt.Printf("[%X]\n", uintptr(4096))
	fmt.Printf("[%+X]\n", uintptr(4096))
	fmt.Printf("[%-X]\n", uintptr(4096))
	fmt.Printf("[% X]\n", uintptr(4096))
	fmt.Printf("[%0X]\n", uintptr(4096))
	fmt.Printf("[%#X]\n", uintptr(4096))
	fmt.Printf("[%8X]\n", uintptr(4096))
	fmt.Printf("[%-8X]\n", uintptr(4096))
	fmt.Printf("[%08X]\n", uintptr(4096))
	fmt.Printf("[%+8X]\n", uintptr(4096))
	fmt.Printf("[%+08X]\n", uintptr(4096))
	fmt.Printf("[% 8X]\n", uintptr(4096))
	fmt.Printf("[%8.3X]\n", uintptr(4096))
	fmt.Printf("[%.3X]\n", uintptr(4096))
	fmt.Printf("[%.0X]\n", uintptr(4096))
	fmt.Printf("[%#8X]\n", uintptr(4096))
	fmt.Printf("[%#08X]\n", uintptr(4096))
	fmt.Printf("[%-#8X]\n", uintptr(4096))
	fmt.Printf("[%+.5X]\n", uintptr(4096))
	fmt.Printf("[%0-8X]\n", uintptr(4096))
	fmt.Printf("[%12.8X]\n", uintptr(4096))
	fmt.Printf("[%-12.8X]\n", uintptr(4096))
	fmt.Printf("[%#.4X]\n", uintptr(4096))
	fmt.Printf("[%+#X]\n", uintptr(4096))
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
// [1000]
// [+1000]
// [1000]
// [ 1000]
// [1000]
// [0X1000]
// [    1000]
// [1000    ]
// [00001000]
// [   +1000]
// [+0001000]
// [    1000]
// [    1000]
// [1000]
// [1000]
// [  0X1000]
// [0X00001000]
// [0X1000  ]
// [+01000]
// [1000    ]
// [    00001000]
// [00001000    ]
// [0X1000]
// [+0X1000]
