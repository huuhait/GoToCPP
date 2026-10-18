// phase: 05

// Go's %x of int64 values with every flag, width and precision: the integer formatting of
// fmt/format.go.
package main

import "fmt"

func main() {
	fmt.Printf("[%x]\n", int64(9223372036854775807))
	fmt.Printf("[%+x]\n", int64(9223372036854775807))
	fmt.Printf("[%-x]\n", int64(9223372036854775807))
	fmt.Printf("[% x]\n", int64(9223372036854775807))
	fmt.Printf("[%0x]\n", int64(9223372036854775807))
	fmt.Printf("[%#x]\n", int64(9223372036854775807))
	fmt.Printf("[%8x]\n", int64(9223372036854775807))
	fmt.Printf("[%-8x]\n", int64(9223372036854775807))
	fmt.Printf("[%08x]\n", int64(9223372036854775807))
	fmt.Printf("[%+8x]\n", int64(9223372036854775807))
	fmt.Printf("[%+08x]\n", int64(9223372036854775807))
	fmt.Printf("[% 8x]\n", int64(9223372036854775807))
	fmt.Printf("[%8.3x]\n", int64(9223372036854775807))
	fmt.Printf("[%.3x]\n", int64(9223372036854775807))
	fmt.Printf("[%.0x]\n", int64(9223372036854775807))
	fmt.Printf("[%#8x]\n", int64(9223372036854775807))
	fmt.Printf("[%#08x]\n", int64(9223372036854775807))
	fmt.Printf("[%-#8x]\n", int64(9223372036854775807))
	fmt.Printf("[%+.5x]\n", int64(9223372036854775807))
	fmt.Printf("[%0-8x]\n", int64(9223372036854775807))
	fmt.Printf("[%12.8x]\n", int64(9223372036854775807))
	fmt.Printf("[%-12.8x]\n", int64(9223372036854775807))
	fmt.Printf("[%#.4x]\n", int64(9223372036854775807))
	fmt.Printf("[%+#x]\n", int64(9223372036854775807))
	fmt.Printf("[%x]\n", int64(-9223372036854775808))
	fmt.Printf("[%+x]\n", int64(-9223372036854775808))
	fmt.Printf("[%-x]\n", int64(-9223372036854775808))
	fmt.Printf("[% x]\n", int64(-9223372036854775808))
	fmt.Printf("[%0x]\n", int64(-9223372036854775808))
	fmt.Printf("[%#x]\n", int64(-9223372036854775808))
	fmt.Printf("[%8x]\n", int64(-9223372036854775808))
	fmt.Printf("[%-8x]\n", int64(-9223372036854775808))
	fmt.Printf("[%08x]\n", int64(-9223372036854775808))
	fmt.Printf("[%+8x]\n", int64(-9223372036854775808))
	fmt.Printf("[%+08x]\n", int64(-9223372036854775808))
	fmt.Printf("[% 8x]\n", int64(-9223372036854775808))
	fmt.Printf("[%8.3x]\n", int64(-9223372036854775808))
	fmt.Printf("[%.3x]\n", int64(-9223372036854775808))
	fmt.Printf("[%.0x]\n", int64(-9223372036854775808))
	fmt.Printf("[%#8x]\n", int64(-9223372036854775808))
	fmt.Printf("[%#08x]\n", int64(-9223372036854775808))
	fmt.Printf("[%-#8x]\n", int64(-9223372036854775808))
	fmt.Printf("[%+.5x]\n", int64(-9223372036854775808))
	fmt.Printf("[%0-8x]\n", int64(-9223372036854775808))
	fmt.Printf("[%12.8x]\n", int64(-9223372036854775808))
	fmt.Printf("[%-12.8x]\n", int64(-9223372036854775808))
	fmt.Printf("[%#.4x]\n", int64(-9223372036854775808))
	fmt.Printf("[%+#x]\n", int64(-9223372036854775808))
	fmt.Printf("[%x]\n", int64(1) << 40)
	fmt.Printf("[%+x]\n", int64(1) << 40)
	fmt.Printf("[%-x]\n", int64(1) << 40)
	fmt.Printf("[% x]\n", int64(1) << 40)
	fmt.Printf("[%0x]\n", int64(1) << 40)
	fmt.Printf("[%#x]\n", int64(1) << 40)
	fmt.Printf("[%8x]\n", int64(1) << 40)
	fmt.Printf("[%-8x]\n", int64(1) << 40)
	fmt.Printf("[%08x]\n", int64(1) << 40)
	fmt.Printf("[%+8x]\n", int64(1) << 40)
	fmt.Printf("[%+08x]\n", int64(1) << 40)
	fmt.Printf("[% 8x]\n", int64(1) << 40)
	fmt.Printf("[%8.3x]\n", int64(1) << 40)
	fmt.Printf("[%.3x]\n", int64(1) << 40)
	fmt.Printf("[%.0x]\n", int64(1) << 40)
	fmt.Printf("[%#8x]\n", int64(1) << 40)
	fmt.Printf("[%#08x]\n", int64(1) << 40)
	fmt.Printf("[%-#8x]\n", int64(1) << 40)
	fmt.Printf("[%+.5x]\n", int64(1) << 40)
	fmt.Printf("[%0-8x]\n", int64(1) << 40)
	fmt.Printf("[%12.8x]\n", int64(1) << 40)
	fmt.Printf("[%-12.8x]\n", int64(1) << 40)
	fmt.Printf("[%#.4x]\n", int64(1) << 40)
	fmt.Printf("[%+#x]\n", int64(1) << 40)
}

// Output:
// [7fffffffffffffff]
// [+7fffffffffffffff]
// [7fffffffffffffff]
// [ 7fffffffffffffff]
// [7fffffffffffffff]
// [0x7fffffffffffffff]
// [7fffffffffffffff]
// [7fffffffffffffff]
// [7fffffffffffffff]
// [+7fffffffffffffff]
// [+7fffffffffffffff]
// [ 7fffffffffffffff]
// [7fffffffffffffff]
// [7fffffffffffffff]
// [7fffffffffffffff]
// [0x7fffffffffffffff]
// [0x7fffffffffffffff]
// [0x7fffffffffffffff]
// [+7fffffffffffffff]
// [7fffffffffffffff]
// [7fffffffffffffff]
// [7fffffffffffffff]
// [0x7fffffffffffffff]
// [+0x7fffffffffffffff]
// [-8000000000000000]
// [-8000000000000000]
// [-8000000000000000]
// [-8000000000000000]
// [-8000000000000000]
// [-0x8000000000000000]
// [-8000000000000000]
// [-8000000000000000]
// [-8000000000000000]
// [-8000000000000000]
// [-8000000000000000]
// [-8000000000000000]
// [-8000000000000000]
// [-8000000000000000]
// [-8000000000000000]
// [-0x8000000000000000]
// [-0x8000000000000000]
// [-0x8000000000000000]
// [-8000000000000000]
// [-8000000000000000]
// [-8000000000000000]
// [-8000000000000000]
// [-0x8000000000000000]
// [-0x8000000000000000]
// [10000000000]
// [+10000000000]
// [10000000000]
// [ 10000000000]
// [10000000000]
// [0x10000000000]
// [10000000000]
// [10000000000]
// [10000000000]
// [+10000000000]
// [+10000000000]
// [ 10000000000]
// [10000000000]
// [10000000000]
// [10000000000]
// [0x10000000000]
// [0x10000000000]
// [0x10000000000]
// [+10000000000]
// [10000000000]
// [ 10000000000]
// [10000000000 ]
// [0x10000000000]
// [+0x10000000000]
