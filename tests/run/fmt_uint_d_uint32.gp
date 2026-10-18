// phase: 05

// Go's %d of uint32 values with flags, width and precision.
package main

import "fmt"

func main() {
	fmt.Printf("[%d]\n", uint32(4294967295))
	fmt.Printf("[%+d]\n", uint32(4294967295))
	fmt.Printf("[%-d]\n", uint32(4294967295))
	fmt.Printf("[% d]\n", uint32(4294967295))
	fmt.Printf("[%0d]\n", uint32(4294967295))
	fmt.Printf("[%#d]\n", uint32(4294967295))
	fmt.Printf("[%8d]\n", uint32(4294967295))
	fmt.Printf("[%-8d]\n", uint32(4294967295))
	fmt.Printf("[%08d]\n", uint32(4294967295))
	fmt.Printf("[%+8d]\n", uint32(4294967295))
	fmt.Printf("[%+08d]\n", uint32(4294967295))
	fmt.Printf("[% 8d]\n", uint32(4294967295))
	fmt.Printf("[%8.3d]\n", uint32(4294967295))
	fmt.Printf("[%.3d]\n", uint32(4294967295))
	fmt.Printf("[%.0d]\n", uint32(4294967295))
	fmt.Printf("[%#8d]\n", uint32(4294967295))
	fmt.Printf("[%#08d]\n", uint32(4294967295))
	fmt.Printf("[%-#8d]\n", uint32(4294967295))
	fmt.Printf("[%+.5d]\n", uint32(4294967295))
	fmt.Printf("[%0-8d]\n", uint32(4294967295))
	fmt.Printf("[%12.8d]\n", uint32(4294967295))
	fmt.Printf("[%-12.8d]\n", uint32(4294967295))
	fmt.Printf("[%#.4d]\n", uint32(4294967295))
	fmt.Printf("[%+#d]\n", uint32(4294967295))
	fmt.Printf("[%d]\n", uint32(65))
	fmt.Printf("[%+d]\n", uint32(65))
	fmt.Printf("[%-d]\n", uint32(65))
	fmt.Printf("[% d]\n", uint32(65))
	fmt.Printf("[%0d]\n", uint32(65))
	fmt.Printf("[%#d]\n", uint32(65))
	fmt.Printf("[%8d]\n", uint32(65))
	fmt.Printf("[%-8d]\n", uint32(65))
	fmt.Printf("[%08d]\n", uint32(65))
	fmt.Printf("[%+8d]\n", uint32(65))
	fmt.Printf("[%+08d]\n", uint32(65))
	fmt.Printf("[% 8d]\n", uint32(65))
	fmt.Printf("[%8.3d]\n", uint32(65))
	fmt.Printf("[%.3d]\n", uint32(65))
	fmt.Printf("[%.0d]\n", uint32(65))
	fmt.Printf("[%#8d]\n", uint32(65))
	fmt.Printf("[%#08d]\n", uint32(65))
	fmt.Printf("[%-#8d]\n", uint32(65))
	fmt.Printf("[%+.5d]\n", uint32(65))
	fmt.Printf("[%0-8d]\n", uint32(65))
	fmt.Printf("[%12.8d]\n", uint32(65))
	fmt.Printf("[%-12.8d]\n", uint32(65))
	fmt.Printf("[%#.4d]\n", uint32(65))
	fmt.Printf("[%+#d]\n", uint32(65))
}

// Output:
// [4294967295]
// [+4294967295]
// [4294967295]
// [ 4294967295]
// [4294967295]
// [4294967295]
// [4294967295]
// [4294967295]
// [4294967295]
// [+4294967295]
// [+4294967295]
// [ 4294967295]
// [4294967295]
// [4294967295]
// [4294967295]
// [4294967295]
// [4294967295]
// [4294967295]
// [+4294967295]
// [4294967295]
// [  4294967295]
// [4294967295  ]
// [4294967295]
// [+4294967295]
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
