// phase: 05

// Go's %d of uintptr values with flags, width and precision.
package main

import "fmt"

func main() {
	fmt.Printf("[%d]\n", uintptr(0))
	fmt.Printf("[%+d]\n", uintptr(0))
	fmt.Printf("[%-d]\n", uintptr(0))
	fmt.Printf("[% d]\n", uintptr(0))
	fmt.Printf("[%0d]\n", uintptr(0))
	fmt.Printf("[%#d]\n", uintptr(0))
	fmt.Printf("[%8d]\n", uintptr(0))
	fmt.Printf("[%-8d]\n", uintptr(0))
	fmt.Printf("[%08d]\n", uintptr(0))
	fmt.Printf("[%+8d]\n", uintptr(0))
	fmt.Printf("[%+08d]\n", uintptr(0))
	fmt.Printf("[% 8d]\n", uintptr(0))
	fmt.Printf("[%8.3d]\n", uintptr(0))
	fmt.Printf("[%.3d]\n", uintptr(0))
	fmt.Printf("[%.0d]\n", uintptr(0))
	fmt.Printf("[%#8d]\n", uintptr(0))
	fmt.Printf("[%#08d]\n", uintptr(0))
	fmt.Printf("[%-#8d]\n", uintptr(0))
	fmt.Printf("[%+.5d]\n", uintptr(0))
	fmt.Printf("[%0-8d]\n", uintptr(0))
	fmt.Printf("[%12.8d]\n", uintptr(0))
	fmt.Printf("[%-12.8d]\n", uintptr(0))
	fmt.Printf("[%#.4d]\n", uintptr(0))
	fmt.Printf("[%+#d]\n", uintptr(0))
	fmt.Printf("[%d]\n", uintptr(4096))
	fmt.Printf("[%+d]\n", uintptr(4096))
	fmt.Printf("[%-d]\n", uintptr(4096))
	fmt.Printf("[% d]\n", uintptr(4096))
	fmt.Printf("[%0d]\n", uintptr(4096))
	fmt.Printf("[%#d]\n", uintptr(4096))
	fmt.Printf("[%8d]\n", uintptr(4096))
	fmt.Printf("[%-8d]\n", uintptr(4096))
	fmt.Printf("[%08d]\n", uintptr(4096))
	fmt.Printf("[%+8d]\n", uintptr(4096))
	fmt.Printf("[%+08d]\n", uintptr(4096))
	fmt.Printf("[% 8d]\n", uintptr(4096))
	fmt.Printf("[%8.3d]\n", uintptr(4096))
	fmt.Printf("[%.3d]\n", uintptr(4096))
	fmt.Printf("[%.0d]\n", uintptr(4096))
	fmt.Printf("[%#8d]\n", uintptr(4096))
	fmt.Printf("[%#08d]\n", uintptr(4096))
	fmt.Printf("[%-#8d]\n", uintptr(4096))
	fmt.Printf("[%+.5d]\n", uintptr(4096))
	fmt.Printf("[%0-8d]\n", uintptr(4096))
	fmt.Printf("[%12.8d]\n", uintptr(4096))
	fmt.Printf("[%-12.8d]\n", uintptr(4096))
	fmt.Printf("[%#.4d]\n", uintptr(4096))
	fmt.Printf("[%+#d]\n", uintptr(4096))
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
// [4096]
// [+4096]
// [4096]
// [ 4096]
// [4096]
// [4096]
// [    4096]
// [4096    ]
// [00004096]
// [   +4096]
// [+0004096]
// [    4096]
// [    4096]
// [4096]
// [4096]
// [    4096]
// [00004096]
// [4096    ]
// [+04096]
// [4096    ]
// [    00004096]
// [00004096    ]
// [4096]
// [+4096]
