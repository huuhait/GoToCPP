// phase: 05

// Go's %d of uint16 values with flags, width and precision.
package main

import "fmt"

func main() {
	fmt.Printf("[%d]\n", uint16(65535))
	fmt.Printf("[%+d]\n", uint16(65535))
	fmt.Printf("[%-d]\n", uint16(65535))
	fmt.Printf("[% d]\n", uint16(65535))
	fmt.Printf("[%0d]\n", uint16(65535))
	fmt.Printf("[%#d]\n", uint16(65535))
	fmt.Printf("[%8d]\n", uint16(65535))
	fmt.Printf("[%-8d]\n", uint16(65535))
	fmt.Printf("[%08d]\n", uint16(65535))
	fmt.Printf("[%+8d]\n", uint16(65535))
	fmt.Printf("[%+08d]\n", uint16(65535))
	fmt.Printf("[% 8d]\n", uint16(65535))
	fmt.Printf("[%8.3d]\n", uint16(65535))
	fmt.Printf("[%.3d]\n", uint16(65535))
	fmt.Printf("[%.0d]\n", uint16(65535))
	fmt.Printf("[%#8d]\n", uint16(65535))
	fmt.Printf("[%#08d]\n", uint16(65535))
	fmt.Printf("[%-#8d]\n", uint16(65535))
	fmt.Printf("[%+.5d]\n", uint16(65535))
	fmt.Printf("[%0-8d]\n", uint16(65535))
	fmt.Printf("[%12.8d]\n", uint16(65535))
	fmt.Printf("[%-12.8d]\n", uint16(65535))
	fmt.Printf("[%#.4d]\n", uint16(65535))
	fmt.Printf("[%+#d]\n", uint16(65535))
	fmt.Printf("[%d]\n", uint16(300))
	fmt.Printf("[%+d]\n", uint16(300))
	fmt.Printf("[%-d]\n", uint16(300))
	fmt.Printf("[% d]\n", uint16(300))
	fmt.Printf("[%0d]\n", uint16(300))
	fmt.Printf("[%#d]\n", uint16(300))
	fmt.Printf("[%8d]\n", uint16(300))
	fmt.Printf("[%-8d]\n", uint16(300))
	fmt.Printf("[%08d]\n", uint16(300))
	fmt.Printf("[%+8d]\n", uint16(300))
	fmt.Printf("[%+08d]\n", uint16(300))
	fmt.Printf("[% 8d]\n", uint16(300))
	fmt.Printf("[%8.3d]\n", uint16(300))
	fmt.Printf("[%.3d]\n", uint16(300))
	fmt.Printf("[%.0d]\n", uint16(300))
	fmt.Printf("[%#8d]\n", uint16(300))
	fmt.Printf("[%#08d]\n", uint16(300))
	fmt.Printf("[%-#8d]\n", uint16(300))
	fmt.Printf("[%+.5d]\n", uint16(300))
	fmt.Printf("[%0-8d]\n", uint16(300))
	fmt.Printf("[%12.8d]\n", uint16(300))
	fmt.Printf("[%-12.8d]\n", uint16(300))
	fmt.Printf("[%#.4d]\n", uint16(300))
	fmt.Printf("[%+#d]\n", uint16(300))
}

// Output:
// [65535]
// [+65535]
// [65535]
// [ 65535]
// [65535]
// [65535]
// [   65535]
// [65535   ]
// [00065535]
// [  +65535]
// [+0065535]
// [   65535]
// [   65535]
// [65535]
// [65535]
// [   65535]
// [00065535]
// [65535   ]
// [+65535]
// [65535   ]
// [    00065535]
// [00065535    ]
// [65535]
// [+65535]
// [300]
// [+300]
// [300]
// [ 300]
// [300]
// [300]
// [     300]
// [300     ]
// [00000300]
// [    +300]
// [+0000300]
// [     300]
// [     300]
// [300]
// [300]
// [     300]
// [00000300]
// [300     ]
// [+00300]
// [300     ]
// [    00000300]
// [00000300    ]
// [0300]
// [+300]
