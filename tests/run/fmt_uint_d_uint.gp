// phase: 05

// Go's %d of uint values with flags, width and precision.
package main

import "fmt"

func main() {
	fmt.Printf("[%d]\n", uint(0))
	fmt.Printf("[%+d]\n", uint(0))
	fmt.Printf("[%-d]\n", uint(0))
	fmt.Printf("[% d]\n", uint(0))
	fmt.Printf("[%0d]\n", uint(0))
	fmt.Printf("[%#d]\n", uint(0))
	fmt.Printf("[%8d]\n", uint(0))
	fmt.Printf("[%-8d]\n", uint(0))
	fmt.Printf("[%08d]\n", uint(0))
	fmt.Printf("[%+8d]\n", uint(0))
	fmt.Printf("[%+08d]\n", uint(0))
	fmt.Printf("[% 8d]\n", uint(0))
	fmt.Printf("[%8.3d]\n", uint(0))
	fmt.Printf("[%.3d]\n", uint(0))
	fmt.Printf("[%.0d]\n", uint(0))
	fmt.Printf("[%#8d]\n", uint(0))
	fmt.Printf("[%#08d]\n", uint(0))
	fmt.Printf("[%-#8d]\n", uint(0))
	fmt.Printf("[%+.5d]\n", uint(0))
	fmt.Printf("[%0-8d]\n", uint(0))
	fmt.Printf("[%12.8d]\n", uint(0))
	fmt.Printf("[%-12.8d]\n", uint(0))
	fmt.Printf("[%#.4d]\n", uint(0))
	fmt.Printf("[%+#d]\n", uint(0))
	fmt.Printf("[%d]\n", uint(1))
	fmt.Printf("[%+d]\n", uint(1))
	fmt.Printf("[%-d]\n", uint(1))
	fmt.Printf("[% d]\n", uint(1))
	fmt.Printf("[%0d]\n", uint(1))
	fmt.Printf("[%#d]\n", uint(1))
	fmt.Printf("[%8d]\n", uint(1))
	fmt.Printf("[%-8d]\n", uint(1))
	fmt.Printf("[%08d]\n", uint(1))
	fmt.Printf("[%+8d]\n", uint(1))
	fmt.Printf("[%+08d]\n", uint(1))
	fmt.Printf("[% 8d]\n", uint(1))
	fmt.Printf("[%8.3d]\n", uint(1))
	fmt.Printf("[%.3d]\n", uint(1))
	fmt.Printf("[%.0d]\n", uint(1))
	fmt.Printf("[%#8d]\n", uint(1))
	fmt.Printf("[%#08d]\n", uint(1))
	fmt.Printf("[%-#8d]\n", uint(1))
	fmt.Printf("[%+.5d]\n", uint(1))
	fmt.Printf("[%0-8d]\n", uint(1))
	fmt.Printf("[%12.8d]\n", uint(1))
	fmt.Printf("[%-12.8d]\n", uint(1))
	fmt.Printf("[%#.4d]\n", uint(1))
	fmt.Printf("[%+#d]\n", uint(1))
	fmt.Printf("[%d]\n", uint(42))
	fmt.Printf("[%+d]\n", uint(42))
	fmt.Printf("[%-d]\n", uint(42))
	fmt.Printf("[% d]\n", uint(42))
	fmt.Printf("[%0d]\n", uint(42))
	fmt.Printf("[%#d]\n", uint(42))
	fmt.Printf("[%8d]\n", uint(42))
	fmt.Printf("[%-8d]\n", uint(42))
	fmt.Printf("[%08d]\n", uint(42))
	fmt.Printf("[%+8d]\n", uint(42))
	fmt.Printf("[%+08d]\n", uint(42))
	fmt.Printf("[% 8d]\n", uint(42))
	fmt.Printf("[%8.3d]\n", uint(42))
	fmt.Printf("[%.3d]\n", uint(42))
	fmt.Printf("[%.0d]\n", uint(42))
	fmt.Printf("[%#8d]\n", uint(42))
	fmt.Printf("[%#08d]\n", uint(42))
	fmt.Printf("[%-#8d]\n", uint(42))
	fmt.Printf("[%+.5d]\n", uint(42))
	fmt.Printf("[%0-8d]\n", uint(42))
	fmt.Printf("[%12.8d]\n", uint(42))
	fmt.Printf("[%-12.8d]\n", uint(42))
	fmt.Printf("[%#.4d]\n", uint(42))
	fmt.Printf("[%+#d]\n", uint(42))
	fmt.Printf("[%d]\n", uint(1234567))
	fmt.Printf("[%+d]\n", uint(1234567))
	fmt.Printf("[%-d]\n", uint(1234567))
	fmt.Printf("[% d]\n", uint(1234567))
	fmt.Printf("[%0d]\n", uint(1234567))
	fmt.Printf("[%#d]\n", uint(1234567))
	fmt.Printf("[%8d]\n", uint(1234567))
	fmt.Printf("[%-8d]\n", uint(1234567))
	fmt.Printf("[%08d]\n", uint(1234567))
	fmt.Printf("[%+8d]\n", uint(1234567))
	fmt.Printf("[%+08d]\n", uint(1234567))
	fmt.Printf("[% 8d]\n", uint(1234567))
	fmt.Printf("[%8.3d]\n", uint(1234567))
	fmt.Printf("[%.3d]\n", uint(1234567))
	fmt.Printf("[%.0d]\n", uint(1234567))
	fmt.Printf("[%#8d]\n", uint(1234567))
	fmt.Printf("[%#08d]\n", uint(1234567))
	fmt.Printf("[%-#8d]\n", uint(1234567))
	fmt.Printf("[%+.5d]\n", uint(1234567))
	fmt.Printf("[%0-8d]\n", uint(1234567))
	fmt.Printf("[%12.8d]\n", uint(1234567))
	fmt.Printf("[%-12.8d]\n", uint(1234567))
	fmt.Printf("[%#.4d]\n", uint(1234567))
	fmt.Printf("[%+#d]\n", uint(1234567))
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
// [1]
// [+1]
// [1]
// [ 1]
// [1]
// [1]
// [       1]
// [1       ]
// [00000001]
// [      +1]
// [+0000001]
// [       1]
// [     001]
// [001]
// [1]
// [       1]
// [00000001]
// [1       ]
// [+00001]
// [1       ]
// [    00000001]
// [00000001    ]
// [0001]
// [+1]
// [42]
// [+42]
// [42]
// [ 42]
// [42]
// [42]
// [      42]
// [42      ]
// [00000042]
// [     +42]
// [+0000042]
// [      42]
// [     042]
// [042]
// [42]
// [      42]
// [00000042]
// [42      ]
// [+00042]
// [42      ]
// [    00000042]
// [00000042    ]
// [0042]
// [+42]
// [1234567]
// [+1234567]
// [1234567]
// [ 1234567]
// [1234567]
// [1234567]
// [ 1234567]
// [1234567 ]
// [01234567]
// [+1234567]
// [+1234567]
// [ 1234567]
// [ 1234567]
// [1234567]
// [1234567]
// [ 1234567]
// [01234567]
// [1234567 ]
// [+1234567]
// [1234567 ]
// [    01234567]
// [01234567    ]
// [1234567]
// [+1234567]
