// phase: 05

// Go's %X of int64 values with every flag, width and precision: the integer formatting of
// fmt/format.go.
package main

import "fmt"

func main() {
	fmt.Printf("[%X]\n", int64(9223372036854775807))
	fmt.Printf("[%+X]\n", int64(9223372036854775807))
	fmt.Printf("[%-X]\n", int64(9223372036854775807))
	fmt.Printf("[% X]\n", int64(9223372036854775807))
	fmt.Printf("[%0X]\n", int64(9223372036854775807))
	fmt.Printf("[%#X]\n", int64(9223372036854775807))
	fmt.Printf("[%8X]\n", int64(9223372036854775807))
	fmt.Printf("[%-8X]\n", int64(9223372036854775807))
	fmt.Printf("[%08X]\n", int64(9223372036854775807))
	fmt.Printf("[%+8X]\n", int64(9223372036854775807))
	fmt.Printf("[%+08X]\n", int64(9223372036854775807))
	fmt.Printf("[% 8X]\n", int64(9223372036854775807))
	fmt.Printf("[%8.3X]\n", int64(9223372036854775807))
	fmt.Printf("[%.3X]\n", int64(9223372036854775807))
	fmt.Printf("[%.0X]\n", int64(9223372036854775807))
	fmt.Printf("[%#8X]\n", int64(9223372036854775807))
	fmt.Printf("[%#08X]\n", int64(9223372036854775807))
	fmt.Printf("[%-#8X]\n", int64(9223372036854775807))
	fmt.Printf("[%+.5X]\n", int64(9223372036854775807))
	fmt.Printf("[%0-8X]\n", int64(9223372036854775807))
	fmt.Printf("[%12.8X]\n", int64(9223372036854775807))
	fmt.Printf("[%-12.8X]\n", int64(9223372036854775807))
	fmt.Printf("[%#.4X]\n", int64(9223372036854775807))
	fmt.Printf("[%+#X]\n", int64(9223372036854775807))
	fmt.Printf("[%X]\n", int64(-9223372036854775808))
	fmt.Printf("[%+X]\n", int64(-9223372036854775808))
	fmt.Printf("[%-X]\n", int64(-9223372036854775808))
	fmt.Printf("[% X]\n", int64(-9223372036854775808))
	fmt.Printf("[%0X]\n", int64(-9223372036854775808))
	fmt.Printf("[%#X]\n", int64(-9223372036854775808))
	fmt.Printf("[%8X]\n", int64(-9223372036854775808))
	fmt.Printf("[%-8X]\n", int64(-9223372036854775808))
	fmt.Printf("[%08X]\n", int64(-9223372036854775808))
	fmt.Printf("[%+8X]\n", int64(-9223372036854775808))
	fmt.Printf("[%+08X]\n", int64(-9223372036854775808))
	fmt.Printf("[% 8X]\n", int64(-9223372036854775808))
	fmt.Printf("[%8.3X]\n", int64(-9223372036854775808))
	fmt.Printf("[%.3X]\n", int64(-9223372036854775808))
	fmt.Printf("[%.0X]\n", int64(-9223372036854775808))
	fmt.Printf("[%#8X]\n", int64(-9223372036854775808))
	fmt.Printf("[%#08X]\n", int64(-9223372036854775808))
	fmt.Printf("[%-#8X]\n", int64(-9223372036854775808))
	fmt.Printf("[%+.5X]\n", int64(-9223372036854775808))
	fmt.Printf("[%0-8X]\n", int64(-9223372036854775808))
	fmt.Printf("[%12.8X]\n", int64(-9223372036854775808))
	fmt.Printf("[%-12.8X]\n", int64(-9223372036854775808))
	fmt.Printf("[%#.4X]\n", int64(-9223372036854775808))
	fmt.Printf("[%+#X]\n", int64(-9223372036854775808))
	fmt.Printf("[%X]\n", int64(1) << 40)
	fmt.Printf("[%+X]\n", int64(1) << 40)
	fmt.Printf("[%-X]\n", int64(1) << 40)
	fmt.Printf("[% X]\n", int64(1) << 40)
	fmt.Printf("[%0X]\n", int64(1) << 40)
	fmt.Printf("[%#X]\n", int64(1) << 40)
	fmt.Printf("[%8X]\n", int64(1) << 40)
	fmt.Printf("[%-8X]\n", int64(1) << 40)
	fmt.Printf("[%08X]\n", int64(1) << 40)
	fmt.Printf("[%+8X]\n", int64(1) << 40)
	fmt.Printf("[%+08X]\n", int64(1) << 40)
	fmt.Printf("[% 8X]\n", int64(1) << 40)
	fmt.Printf("[%8.3X]\n", int64(1) << 40)
	fmt.Printf("[%.3X]\n", int64(1) << 40)
	fmt.Printf("[%.0X]\n", int64(1) << 40)
	fmt.Printf("[%#8X]\n", int64(1) << 40)
	fmt.Printf("[%#08X]\n", int64(1) << 40)
	fmt.Printf("[%-#8X]\n", int64(1) << 40)
	fmt.Printf("[%+.5X]\n", int64(1) << 40)
	fmt.Printf("[%0-8X]\n", int64(1) << 40)
	fmt.Printf("[%12.8X]\n", int64(1) << 40)
	fmt.Printf("[%-12.8X]\n", int64(1) << 40)
	fmt.Printf("[%#.4X]\n", int64(1) << 40)
	fmt.Printf("[%+#X]\n", int64(1) << 40)
}

// Output:
// [7FFFFFFFFFFFFFFF]
// [+7FFFFFFFFFFFFFFF]
// [7FFFFFFFFFFFFFFF]
// [ 7FFFFFFFFFFFFFFF]
// [7FFFFFFFFFFFFFFF]
// [0X7FFFFFFFFFFFFFFF]
// [7FFFFFFFFFFFFFFF]
// [7FFFFFFFFFFFFFFF]
// [7FFFFFFFFFFFFFFF]
// [+7FFFFFFFFFFFFFFF]
// [+7FFFFFFFFFFFFFFF]
// [ 7FFFFFFFFFFFFFFF]
// [7FFFFFFFFFFFFFFF]
// [7FFFFFFFFFFFFFFF]
// [7FFFFFFFFFFFFFFF]
// [0X7FFFFFFFFFFFFFFF]
// [0X7FFFFFFFFFFFFFFF]
// [0X7FFFFFFFFFFFFFFF]
// [+7FFFFFFFFFFFFFFF]
// [7FFFFFFFFFFFFFFF]
// [7FFFFFFFFFFFFFFF]
// [7FFFFFFFFFFFFFFF]
// [0X7FFFFFFFFFFFFFFF]
// [+0X7FFFFFFFFFFFFFFF]
// [-8000000000000000]
// [-8000000000000000]
// [-8000000000000000]
// [-8000000000000000]
// [-8000000000000000]
// [-0X8000000000000000]
// [-8000000000000000]
// [-8000000000000000]
// [-8000000000000000]
// [-8000000000000000]
// [-8000000000000000]
// [-8000000000000000]
// [-8000000000000000]
// [-8000000000000000]
// [-8000000000000000]
// [-0X8000000000000000]
// [-0X8000000000000000]
// [-0X8000000000000000]
// [-8000000000000000]
// [-8000000000000000]
// [-8000000000000000]
// [-8000000000000000]
// [-0X8000000000000000]
// [-0X8000000000000000]
// [10000000000]
// [+10000000000]
// [10000000000]
// [ 10000000000]
// [10000000000]
// [0X10000000000]
// [10000000000]
// [10000000000]
// [10000000000]
// [+10000000000]
// [+10000000000]
// [ 10000000000]
// [10000000000]
// [10000000000]
// [10000000000]
// [0X10000000000]
// [0X10000000000]
// [0X10000000000]
// [+10000000000]
// [10000000000]
// [ 10000000000]
// [10000000000 ]
// [0X10000000000]
// [+0X10000000000]
