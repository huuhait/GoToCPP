// phase: 05

// Go's %x of int32 values with every flag, width and precision: the integer formatting of
// fmt/format.go.
package main

import "fmt"

func main() {
	fmt.Printf("[%x]\n", int32(2147483647))
	fmt.Printf("[%+x]\n", int32(2147483647))
	fmt.Printf("[%-x]\n", int32(2147483647))
	fmt.Printf("[% x]\n", int32(2147483647))
	fmt.Printf("[%0x]\n", int32(2147483647))
	fmt.Printf("[%#x]\n", int32(2147483647))
	fmt.Printf("[%8x]\n", int32(2147483647))
	fmt.Printf("[%-8x]\n", int32(2147483647))
	fmt.Printf("[%08x]\n", int32(2147483647))
	fmt.Printf("[%+8x]\n", int32(2147483647))
	fmt.Printf("[%+08x]\n", int32(2147483647))
	fmt.Printf("[% 8x]\n", int32(2147483647))
	fmt.Printf("[%8.3x]\n", int32(2147483647))
	fmt.Printf("[%.3x]\n", int32(2147483647))
	fmt.Printf("[%.0x]\n", int32(2147483647))
	fmt.Printf("[%#8x]\n", int32(2147483647))
	fmt.Printf("[%#08x]\n", int32(2147483647))
	fmt.Printf("[%-#8x]\n", int32(2147483647))
	fmt.Printf("[%+.5x]\n", int32(2147483647))
	fmt.Printf("[%0-8x]\n", int32(2147483647))
	fmt.Printf("[%12.8x]\n", int32(2147483647))
	fmt.Printf("[%-12.8x]\n", int32(2147483647))
	fmt.Printf("[%#.4x]\n", int32(2147483647))
	fmt.Printf("[%+#x]\n", int32(2147483647))
	fmt.Printf("[%x]\n", int32(-2147483648))
	fmt.Printf("[%+x]\n", int32(-2147483648))
	fmt.Printf("[%-x]\n", int32(-2147483648))
	fmt.Printf("[% x]\n", int32(-2147483648))
	fmt.Printf("[%0x]\n", int32(-2147483648))
	fmt.Printf("[%#x]\n", int32(-2147483648))
	fmt.Printf("[%8x]\n", int32(-2147483648))
	fmt.Printf("[%-8x]\n", int32(-2147483648))
	fmt.Printf("[%08x]\n", int32(-2147483648))
	fmt.Printf("[%+8x]\n", int32(-2147483648))
	fmt.Printf("[%+08x]\n", int32(-2147483648))
	fmt.Printf("[% 8x]\n", int32(-2147483648))
	fmt.Printf("[%8.3x]\n", int32(-2147483648))
	fmt.Printf("[%.3x]\n", int32(-2147483648))
	fmt.Printf("[%.0x]\n", int32(-2147483648))
	fmt.Printf("[%#8x]\n", int32(-2147483648))
	fmt.Printf("[%#08x]\n", int32(-2147483648))
	fmt.Printf("[%-#8x]\n", int32(-2147483648))
	fmt.Printf("[%+.5x]\n", int32(-2147483648))
	fmt.Printf("[%0-8x]\n", int32(-2147483648))
	fmt.Printf("[%12.8x]\n", int32(-2147483648))
	fmt.Printf("[%-12.8x]\n", int32(-2147483648))
	fmt.Printf("[%#.4x]\n", int32(-2147483648))
	fmt.Printf("[%+#x]\n", int32(-2147483648))
	fmt.Printf("[%x]\n", int32(65))
	fmt.Printf("[%+x]\n", int32(65))
	fmt.Printf("[%-x]\n", int32(65))
	fmt.Printf("[% x]\n", int32(65))
	fmt.Printf("[%0x]\n", int32(65))
	fmt.Printf("[%#x]\n", int32(65))
	fmt.Printf("[%8x]\n", int32(65))
	fmt.Printf("[%-8x]\n", int32(65))
	fmt.Printf("[%08x]\n", int32(65))
	fmt.Printf("[%+8x]\n", int32(65))
	fmt.Printf("[%+08x]\n", int32(65))
	fmt.Printf("[% 8x]\n", int32(65))
	fmt.Printf("[%8.3x]\n", int32(65))
	fmt.Printf("[%.3x]\n", int32(65))
	fmt.Printf("[%.0x]\n", int32(65))
	fmt.Printf("[%#8x]\n", int32(65))
	fmt.Printf("[%#08x]\n", int32(65))
	fmt.Printf("[%-#8x]\n", int32(65))
	fmt.Printf("[%+.5x]\n", int32(65))
	fmt.Printf("[%0-8x]\n", int32(65))
	fmt.Printf("[%12.8x]\n", int32(65))
	fmt.Printf("[%-12.8x]\n", int32(65))
	fmt.Printf("[%#.4x]\n", int32(65))
	fmt.Printf("[%+#x]\n", int32(65))
}

// Output:
// [7fffffff]
// [+7fffffff]
// [7fffffff]
// [ 7fffffff]
// [7fffffff]
// [0x7fffffff]
// [7fffffff]
// [7fffffff]
// [7fffffff]
// [+7fffffff]
// [+7fffffff]
// [ 7fffffff]
// [7fffffff]
// [7fffffff]
// [7fffffff]
// [0x7fffffff]
// [0x7fffffff]
// [0x7fffffff]
// [+7fffffff]
// [7fffffff]
// [    7fffffff]
// [7fffffff    ]
// [0x7fffffff]
// [+0x7fffffff]
// [-80000000]
// [-80000000]
// [-80000000]
// [-80000000]
// [-80000000]
// [-0x80000000]
// [-80000000]
// [-80000000]
// [-80000000]
// [-80000000]
// [-80000000]
// [-80000000]
// [-80000000]
// [-80000000]
// [-80000000]
// [-0x80000000]
// [-0x80000000]
// [-0x80000000]
// [-80000000]
// [-80000000]
// [   -80000000]
// [-80000000   ]
// [-0x80000000]
// [-0x80000000]
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
