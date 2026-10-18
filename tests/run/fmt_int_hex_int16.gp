// phase: 05

// Go's %x of int16 values with every flag, width and precision: the integer formatting of
// fmt/format.go.
package main

import "fmt"

func main() {
	fmt.Printf("[%x]\n", int16(32767))
	fmt.Printf("[%+x]\n", int16(32767))
	fmt.Printf("[%-x]\n", int16(32767))
	fmt.Printf("[% x]\n", int16(32767))
	fmt.Printf("[%0x]\n", int16(32767))
	fmt.Printf("[%#x]\n", int16(32767))
	fmt.Printf("[%8x]\n", int16(32767))
	fmt.Printf("[%-8x]\n", int16(32767))
	fmt.Printf("[%08x]\n", int16(32767))
	fmt.Printf("[%+8x]\n", int16(32767))
	fmt.Printf("[%+08x]\n", int16(32767))
	fmt.Printf("[% 8x]\n", int16(32767))
	fmt.Printf("[%8.3x]\n", int16(32767))
	fmt.Printf("[%.3x]\n", int16(32767))
	fmt.Printf("[%.0x]\n", int16(32767))
	fmt.Printf("[%#8x]\n", int16(32767))
	fmt.Printf("[%#08x]\n", int16(32767))
	fmt.Printf("[%-#8x]\n", int16(32767))
	fmt.Printf("[%+.5x]\n", int16(32767))
	fmt.Printf("[%0-8x]\n", int16(32767))
	fmt.Printf("[%12.8x]\n", int16(32767))
	fmt.Printf("[%-12.8x]\n", int16(32767))
	fmt.Printf("[%#.4x]\n", int16(32767))
	fmt.Printf("[%+#x]\n", int16(32767))
	fmt.Printf("[%x]\n", int16(-32768))
	fmt.Printf("[%+x]\n", int16(-32768))
	fmt.Printf("[%-x]\n", int16(-32768))
	fmt.Printf("[% x]\n", int16(-32768))
	fmt.Printf("[%0x]\n", int16(-32768))
	fmt.Printf("[%#x]\n", int16(-32768))
	fmt.Printf("[%8x]\n", int16(-32768))
	fmt.Printf("[%-8x]\n", int16(-32768))
	fmt.Printf("[%08x]\n", int16(-32768))
	fmt.Printf("[%+8x]\n", int16(-32768))
	fmt.Printf("[%+08x]\n", int16(-32768))
	fmt.Printf("[% 8x]\n", int16(-32768))
	fmt.Printf("[%8.3x]\n", int16(-32768))
	fmt.Printf("[%.3x]\n", int16(-32768))
	fmt.Printf("[%.0x]\n", int16(-32768))
	fmt.Printf("[%#8x]\n", int16(-32768))
	fmt.Printf("[%#08x]\n", int16(-32768))
	fmt.Printf("[%-#8x]\n", int16(-32768))
	fmt.Printf("[%+.5x]\n", int16(-32768))
	fmt.Printf("[%0-8x]\n", int16(-32768))
	fmt.Printf("[%12.8x]\n", int16(-32768))
	fmt.Printf("[%-12.8x]\n", int16(-32768))
	fmt.Printf("[%#.4x]\n", int16(-32768))
	fmt.Printf("[%+#x]\n", int16(-32768))
	fmt.Printf("[%x]\n", int16(300))
	fmt.Printf("[%+x]\n", int16(300))
	fmt.Printf("[%-x]\n", int16(300))
	fmt.Printf("[% x]\n", int16(300))
	fmt.Printf("[%0x]\n", int16(300))
	fmt.Printf("[%#x]\n", int16(300))
	fmt.Printf("[%8x]\n", int16(300))
	fmt.Printf("[%-8x]\n", int16(300))
	fmt.Printf("[%08x]\n", int16(300))
	fmt.Printf("[%+8x]\n", int16(300))
	fmt.Printf("[%+08x]\n", int16(300))
	fmt.Printf("[% 8x]\n", int16(300))
	fmt.Printf("[%8.3x]\n", int16(300))
	fmt.Printf("[%.3x]\n", int16(300))
	fmt.Printf("[%.0x]\n", int16(300))
	fmt.Printf("[%#8x]\n", int16(300))
	fmt.Printf("[%#08x]\n", int16(300))
	fmt.Printf("[%-#8x]\n", int16(300))
	fmt.Printf("[%+.5x]\n", int16(300))
	fmt.Printf("[%0-8x]\n", int16(300))
	fmt.Printf("[%12.8x]\n", int16(300))
	fmt.Printf("[%-12.8x]\n", int16(300))
	fmt.Printf("[%#.4x]\n", int16(300))
	fmt.Printf("[%+#x]\n", int16(300))
}

// Output:
// [7fff]
// [+7fff]
// [7fff]
// [ 7fff]
// [7fff]
// [0x7fff]
// [    7fff]
// [7fff    ]
// [00007fff]
// [   +7fff]
// [+0007fff]
// [    7fff]
// [    7fff]
// [7fff]
// [7fff]
// [  0x7fff]
// [0x00007fff]
// [0x7fff  ]
// [+07fff]
// [7fff    ]
// [    00007fff]
// [00007fff    ]
// [0x7fff]
// [+0x7fff]
// [-8000]
// [-8000]
// [-8000]
// [-8000]
// [-8000]
// [-0x8000]
// [   -8000]
// [-8000   ]
// [-0008000]
// [   -8000]
// [-0008000]
// [   -8000]
// [   -8000]
// [-8000]
// [-8000]
// [ -0x8000]
// [-0x0008000]
// [-0x8000 ]
// [-08000]
// [-8000   ]
// [   -00008000]
// [-00008000   ]
// [-0x8000]
// [-0x8000]
// [12c]
// [+12c]
// [12c]
// [ 12c]
// [12c]
// [0x12c]
// [     12c]
// [12c     ]
// [0000012c]
// [    +12c]
// [+000012c]
// [     12c]
// [     12c]
// [12c]
// [12c]
// [   0x12c]
// [0x0000012c]
// [0x12c   ]
// [+0012c]
// [12c     ]
// [    0000012c]
// [0000012c    ]
// [0x012c]
// [+0x12c]
