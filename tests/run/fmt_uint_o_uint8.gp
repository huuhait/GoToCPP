// phase: 05

// Go's %o of uint8 values with flags, width and precision.
package main

import "fmt"

func main() {
	fmt.Printf("[%o]\n", uint8(0))
	fmt.Printf("[%+o]\n", uint8(0))
	fmt.Printf("[%-o]\n", uint8(0))
	fmt.Printf("[% o]\n", uint8(0))
	fmt.Printf("[%0o]\n", uint8(0))
	fmt.Printf("[%#o]\n", uint8(0))
	fmt.Printf("[%8o]\n", uint8(0))
	fmt.Printf("[%-8o]\n", uint8(0))
	fmt.Printf("[%08o]\n", uint8(0))
	fmt.Printf("[%+8o]\n", uint8(0))
	fmt.Printf("[%+08o]\n", uint8(0))
	fmt.Printf("[% 8o]\n", uint8(0))
	fmt.Printf("[%8.3o]\n", uint8(0))
	fmt.Printf("[%.3o]\n", uint8(0))
	fmt.Printf("[%.0o]\n", uint8(0))
	fmt.Printf("[%#8o]\n", uint8(0))
	fmt.Printf("[%#08o]\n", uint8(0))
	fmt.Printf("[%-#8o]\n", uint8(0))
	fmt.Printf("[%+.5o]\n", uint8(0))
	fmt.Printf("[%0-8o]\n", uint8(0))
	fmt.Printf("[%12.8o]\n", uint8(0))
	fmt.Printf("[%-12.8o]\n", uint8(0))
	fmt.Printf("[%#.4o]\n", uint8(0))
	fmt.Printf("[%+#o]\n", uint8(0))
	fmt.Printf("[%o]\n", uint8(255))
	fmt.Printf("[%+o]\n", uint8(255))
	fmt.Printf("[%-o]\n", uint8(255))
	fmt.Printf("[% o]\n", uint8(255))
	fmt.Printf("[%0o]\n", uint8(255))
	fmt.Printf("[%#o]\n", uint8(255))
	fmt.Printf("[%8o]\n", uint8(255))
	fmt.Printf("[%-8o]\n", uint8(255))
	fmt.Printf("[%08o]\n", uint8(255))
	fmt.Printf("[%+8o]\n", uint8(255))
	fmt.Printf("[%+08o]\n", uint8(255))
	fmt.Printf("[% 8o]\n", uint8(255))
	fmt.Printf("[%8.3o]\n", uint8(255))
	fmt.Printf("[%.3o]\n", uint8(255))
	fmt.Printf("[%.0o]\n", uint8(255))
	fmt.Printf("[%#8o]\n", uint8(255))
	fmt.Printf("[%#08o]\n", uint8(255))
	fmt.Printf("[%-#8o]\n", uint8(255))
	fmt.Printf("[%+.5o]\n", uint8(255))
	fmt.Printf("[%0-8o]\n", uint8(255))
	fmt.Printf("[%12.8o]\n", uint8(255))
	fmt.Printf("[%-12.8o]\n", uint8(255))
	fmt.Printf("[%#.4o]\n", uint8(255))
	fmt.Printf("[%+#o]\n", uint8(255))
	fmt.Printf("[%o]\n", uint8(65))
	fmt.Printf("[%+o]\n", uint8(65))
	fmt.Printf("[%-o]\n", uint8(65))
	fmt.Printf("[% o]\n", uint8(65))
	fmt.Printf("[%0o]\n", uint8(65))
	fmt.Printf("[%#o]\n", uint8(65))
	fmt.Printf("[%8o]\n", uint8(65))
	fmt.Printf("[%-8o]\n", uint8(65))
	fmt.Printf("[%08o]\n", uint8(65))
	fmt.Printf("[%+8o]\n", uint8(65))
	fmt.Printf("[%+08o]\n", uint8(65))
	fmt.Printf("[% 8o]\n", uint8(65))
	fmt.Printf("[%8.3o]\n", uint8(65))
	fmt.Printf("[%.3o]\n", uint8(65))
	fmt.Printf("[%.0o]\n", uint8(65))
	fmt.Printf("[%#8o]\n", uint8(65))
	fmt.Printf("[%#08o]\n", uint8(65))
	fmt.Printf("[%-#8o]\n", uint8(65))
	fmt.Printf("[%+.5o]\n", uint8(65))
	fmt.Printf("[%0-8o]\n", uint8(65))
	fmt.Printf("[%12.8o]\n", uint8(65))
	fmt.Printf("[%-12.8o]\n", uint8(65))
	fmt.Printf("[%#.4o]\n", uint8(65))
	fmt.Printf("[%+#o]\n", uint8(65))
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
// [377]
// [+377]
// [377]
// [ 377]
// [377]
// [0377]
// [     377]
// [377     ]
// [00000377]
// [    +377]
// [+0000377]
// [     377]
// [     377]
// [377]
// [377]
// [    0377]
// [00000377]
// [0377    ]
// [+00377]
// [377     ]
// [    00000377]
// [00000377    ]
// [0377]
// [+0377]
// [101]
// [+101]
// [101]
// [ 101]
// [101]
// [0101]
// [     101]
// [101     ]
// [00000101]
// [    +101]
// [+0000101]
// [     101]
// [     101]
// [101]
// [101]
// [    0101]
// [00000101]
// [0101    ]
// [+00101]
// [101     ]
// [    00000101]
// [00000101    ]
// [0101]
// [+0101]
