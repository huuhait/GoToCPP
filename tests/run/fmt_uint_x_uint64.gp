// phase: 05

// Go's %x of uint64 values with flags, width and precision.
package main

import "fmt"

func main() {
	fmt.Printf("[%x]\n", uint64(18446744073709551615))
	fmt.Printf("[%+x]\n", uint64(18446744073709551615))
	fmt.Printf("[%-x]\n", uint64(18446744073709551615))
	fmt.Printf("[% x]\n", uint64(18446744073709551615))
	fmt.Printf("[%0x]\n", uint64(18446744073709551615))
	fmt.Printf("[%#x]\n", uint64(18446744073709551615))
	fmt.Printf("[%8x]\n", uint64(18446744073709551615))
	fmt.Printf("[%-8x]\n", uint64(18446744073709551615))
	fmt.Printf("[%08x]\n", uint64(18446744073709551615))
	fmt.Printf("[%+8x]\n", uint64(18446744073709551615))
	fmt.Printf("[%+08x]\n", uint64(18446744073709551615))
	fmt.Printf("[% 8x]\n", uint64(18446744073709551615))
	fmt.Printf("[%8.3x]\n", uint64(18446744073709551615))
	fmt.Printf("[%.3x]\n", uint64(18446744073709551615))
	fmt.Printf("[%.0x]\n", uint64(18446744073709551615))
	fmt.Printf("[%#8x]\n", uint64(18446744073709551615))
	fmt.Printf("[%#08x]\n", uint64(18446744073709551615))
	fmt.Printf("[%-#8x]\n", uint64(18446744073709551615))
	fmt.Printf("[%+.5x]\n", uint64(18446744073709551615))
	fmt.Printf("[%0-8x]\n", uint64(18446744073709551615))
	fmt.Printf("[%12.8x]\n", uint64(18446744073709551615))
	fmt.Printf("[%-12.8x]\n", uint64(18446744073709551615))
	fmt.Printf("[%#.4x]\n", uint64(18446744073709551615))
	fmt.Printf("[%+#x]\n", uint64(18446744073709551615))
	fmt.Printf("[%x]\n", uint64(1)<<40)
	fmt.Printf("[%+x]\n", uint64(1)<<40)
	fmt.Printf("[%-x]\n", uint64(1)<<40)
	fmt.Printf("[% x]\n", uint64(1)<<40)
	fmt.Printf("[%0x]\n", uint64(1)<<40)
	fmt.Printf("[%#x]\n", uint64(1)<<40)
	fmt.Printf("[%8x]\n", uint64(1)<<40)
	fmt.Printf("[%-8x]\n", uint64(1)<<40)
	fmt.Printf("[%08x]\n", uint64(1)<<40)
	fmt.Printf("[%+8x]\n", uint64(1)<<40)
	fmt.Printf("[%+08x]\n", uint64(1)<<40)
	fmt.Printf("[% 8x]\n", uint64(1)<<40)
	fmt.Printf("[%8.3x]\n", uint64(1)<<40)
	fmt.Printf("[%.3x]\n", uint64(1)<<40)
	fmt.Printf("[%.0x]\n", uint64(1)<<40)
	fmt.Printf("[%#8x]\n", uint64(1)<<40)
	fmt.Printf("[%#08x]\n", uint64(1)<<40)
	fmt.Printf("[%-#8x]\n", uint64(1)<<40)
	fmt.Printf("[%+.5x]\n", uint64(1)<<40)
	fmt.Printf("[%0-8x]\n", uint64(1)<<40)
	fmt.Printf("[%12.8x]\n", uint64(1)<<40)
	fmt.Printf("[%-12.8x]\n", uint64(1)<<40)
	fmt.Printf("[%#.4x]\n", uint64(1)<<40)
	fmt.Printf("[%+#x]\n", uint64(1)<<40)
}

// Output:
// [ffffffffffffffff]
// [+ffffffffffffffff]
// [ffffffffffffffff]
// [ ffffffffffffffff]
// [ffffffffffffffff]
// [0xffffffffffffffff]
// [ffffffffffffffff]
// [ffffffffffffffff]
// [ffffffffffffffff]
// [+ffffffffffffffff]
// [+ffffffffffffffff]
// [ ffffffffffffffff]
// [ffffffffffffffff]
// [ffffffffffffffff]
// [ffffffffffffffff]
// [0xffffffffffffffff]
// [0xffffffffffffffff]
// [0xffffffffffffffff]
// [+ffffffffffffffff]
// [ffffffffffffffff]
// [ffffffffffffffff]
// [ffffffffffffffff]
// [0xffffffffffffffff]
// [+0xffffffffffffffff]
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
