// phase: 05

// Go's %X of uint64 values with flags, width and precision.
package main

import "fmt"

func main() {
	fmt.Printf("[%X]\n", uint64(18446744073709551615))
	fmt.Printf("[%+X]\n", uint64(18446744073709551615))
	fmt.Printf("[%-X]\n", uint64(18446744073709551615))
	fmt.Printf("[% X]\n", uint64(18446744073709551615))
	fmt.Printf("[%0X]\n", uint64(18446744073709551615))
	fmt.Printf("[%#X]\n", uint64(18446744073709551615))
	fmt.Printf("[%8X]\n", uint64(18446744073709551615))
	fmt.Printf("[%-8X]\n", uint64(18446744073709551615))
	fmt.Printf("[%08X]\n", uint64(18446744073709551615))
	fmt.Printf("[%+8X]\n", uint64(18446744073709551615))
	fmt.Printf("[%+08X]\n", uint64(18446744073709551615))
	fmt.Printf("[% 8X]\n", uint64(18446744073709551615))
	fmt.Printf("[%8.3X]\n", uint64(18446744073709551615))
	fmt.Printf("[%.3X]\n", uint64(18446744073709551615))
	fmt.Printf("[%.0X]\n", uint64(18446744073709551615))
	fmt.Printf("[%#8X]\n", uint64(18446744073709551615))
	fmt.Printf("[%#08X]\n", uint64(18446744073709551615))
	fmt.Printf("[%-#8X]\n", uint64(18446744073709551615))
	fmt.Printf("[%+.5X]\n", uint64(18446744073709551615))
	fmt.Printf("[%0-8X]\n", uint64(18446744073709551615))
	fmt.Printf("[%12.8X]\n", uint64(18446744073709551615))
	fmt.Printf("[%-12.8X]\n", uint64(18446744073709551615))
	fmt.Printf("[%#.4X]\n", uint64(18446744073709551615))
	fmt.Printf("[%+#X]\n", uint64(18446744073709551615))
	fmt.Printf("[%X]\n", uint64(1)<<40)
	fmt.Printf("[%+X]\n", uint64(1)<<40)
	fmt.Printf("[%-X]\n", uint64(1)<<40)
	fmt.Printf("[% X]\n", uint64(1)<<40)
	fmt.Printf("[%0X]\n", uint64(1)<<40)
	fmt.Printf("[%#X]\n", uint64(1)<<40)
	fmt.Printf("[%8X]\n", uint64(1)<<40)
	fmt.Printf("[%-8X]\n", uint64(1)<<40)
	fmt.Printf("[%08X]\n", uint64(1)<<40)
	fmt.Printf("[%+8X]\n", uint64(1)<<40)
	fmt.Printf("[%+08X]\n", uint64(1)<<40)
	fmt.Printf("[% 8X]\n", uint64(1)<<40)
	fmt.Printf("[%8.3X]\n", uint64(1)<<40)
	fmt.Printf("[%.3X]\n", uint64(1)<<40)
	fmt.Printf("[%.0X]\n", uint64(1)<<40)
	fmt.Printf("[%#8X]\n", uint64(1)<<40)
	fmt.Printf("[%#08X]\n", uint64(1)<<40)
	fmt.Printf("[%-#8X]\n", uint64(1)<<40)
	fmt.Printf("[%+.5X]\n", uint64(1)<<40)
	fmt.Printf("[%0-8X]\n", uint64(1)<<40)
	fmt.Printf("[%12.8X]\n", uint64(1)<<40)
	fmt.Printf("[%-12.8X]\n", uint64(1)<<40)
	fmt.Printf("[%#.4X]\n", uint64(1)<<40)
	fmt.Printf("[%+#X]\n", uint64(1)<<40)
}

// Output:
// [FFFFFFFFFFFFFFFF]
// [+FFFFFFFFFFFFFFFF]
// [FFFFFFFFFFFFFFFF]
// [ FFFFFFFFFFFFFFFF]
// [FFFFFFFFFFFFFFFF]
// [0XFFFFFFFFFFFFFFFF]
// [FFFFFFFFFFFFFFFF]
// [FFFFFFFFFFFFFFFF]
// [FFFFFFFFFFFFFFFF]
// [+FFFFFFFFFFFFFFFF]
// [+FFFFFFFFFFFFFFFF]
// [ FFFFFFFFFFFFFFFF]
// [FFFFFFFFFFFFFFFF]
// [FFFFFFFFFFFFFFFF]
// [FFFFFFFFFFFFFFFF]
// [0XFFFFFFFFFFFFFFFF]
// [0XFFFFFFFFFFFFFFFF]
// [0XFFFFFFFFFFFFFFFF]
// [+FFFFFFFFFFFFFFFF]
// [FFFFFFFFFFFFFFFF]
// [FFFFFFFFFFFFFFFF]
// [FFFFFFFFFFFFFFFF]
// [0XFFFFFFFFFFFFFFFF]
// [+0XFFFFFFFFFFFFFFFF]
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
