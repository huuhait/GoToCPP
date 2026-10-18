// phase: 05

// Go's %o of uint values with flags, width and precision.
package main

import "fmt"

func main() {
	fmt.Printf("[%o]\n", uint(0))
	fmt.Printf("[%+o]\n", uint(0))
	fmt.Printf("[%-o]\n", uint(0))
	fmt.Printf("[% o]\n", uint(0))
	fmt.Printf("[%0o]\n", uint(0))
	fmt.Printf("[%#o]\n", uint(0))
	fmt.Printf("[%8o]\n", uint(0))
	fmt.Printf("[%-8o]\n", uint(0))
	fmt.Printf("[%08o]\n", uint(0))
	fmt.Printf("[%+8o]\n", uint(0))
	fmt.Printf("[%+08o]\n", uint(0))
	fmt.Printf("[% 8o]\n", uint(0))
	fmt.Printf("[%8.3o]\n", uint(0))
	fmt.Printf("[%.3o]\n", uint(0))
	fmt.Printf("[%.0o]\n", uint(0))
	fmt.Printf("[%#8o]\n", uint(0))
	fmt.Printf("[%#08o]\n", uint(0))
	fmt.Printf("[%-#8o]\n", uint(0))
	fmt.Printf("[%+.5o]\n", uint(0))
	fmt.Printf("[%0-8o]\n", uint(0))
	fmt.Printf("[%12.8o]\n", uint(0))
	fmt.Printf("[%-12.8o]\n", uint(0))
	fmt.Printf("[%#.4o]\n", uint(0))
	fmt.Printf("[%+#o]\n", uint(0))
	fmt.Printf("[%o]\n", uint(1))
	fmt.Printf("[%+o]\n", uint(1))
	fmt.Printf("[%-o]\n", uint(1))
	fmt.Printf("[% o]\n", uint(1))
	fmt.Printf("[%0o]\n", uint(1))
	fmt.Printf("[%#o]\n", uint(1))
	fmt.Printf("[%8o]\n", uint(1))
	fmt.Printf("[%-8o]\n", uint(1))
	fmt.Printf("[%08o]\n", uint(1))
	fmt.Printf("[%+8o]\n", uint(1))
	fmt.Printf("[%+08o]\n", uint(1))
	fmt.Printf("[% 8o]\n", uint(1))
	fmt.Printf("[%8.3o]\n", uint(1))
	fmt.Printf("[%.3o]\n", uint(1))
	fmt.Printf("[%.0o]\n", uint(1))
	fmt.Printf("[%#8o]\n", uint(1))
	fmt.Printf("[%#08o]\n", uint(1))
	fmt.Printf("[%-#8o]\n", uint(1))
	fmt.Printf("[%+.5o]\n", uint(1))
	fmt.Printf("[%0-8o]\n", uint(1))
	fmt.Printf("[%12.8o]\n", uint(1))
	fmt.Printf("[%-12.8o]\n", uint(1))
	fmt.Printf("[%#.4o]\n", uint(1))
	fmt.Printf("[%+#o]\n", uint(1))
	fmt.Printf("[%o]\n", uint(42))
	fmt.Printf("[%+o]\n", uint(42))
	fmt.Printf("[%-o]\n", uint(42))
	fmt.Printf("[% o]\n", uint(42))
	fmt.Printf("[%0o]\n", uint(42))
	fmt.Printf("[%#o]\n", uint(42))
	fmt.Printf("[%8o]\n", uint(42))
	fmt.Printf("[%-8o]\n", uint(42))
	fmt.Printf("[%08o]\n", uint(42))
	fmt.Printf("[%+8o]\n", uint(42))
	fmt.Printf("[%+08o]\n", uint(42))
	fmt.Printf("[% 8o]\n", uint(42))
	fmt.Printf("[%8.3o]\n", uint(42))
	fmt.Printf("[%.3o]\n", uint(42))
	fmt.Printf("[%.0o]\n", uint(42))
	fmt.Printf("[%#8o]\n", uint(42))
	fmt.Printf("[%#08o]\n", uint(42))
	fmt.Printf("[%-#8o]\n", uint(42))
	fmt.Printf("[%+.5o]\n", uint(42))
	fmt.Printf("[%0-8o]\n", uint(42))
	fmt.Printf("[%12.8o]\n", uint(42))
	fmt.Printf("[%-12.8o]\n", uint(42))
	fmt.Printf("[%#.4o]\n", uint(42))
	fmt.Printf("[%+#o]\n", uint(42))
	fmt.Printf("[%o]\n", uint(1234567))
	fmt.Printf("[%+o]\n", uint(1234567))
	fmt.Printf("[%-o]\n", uint(1234567))
	fmt.Printf("[% o]\n", uint(1234567))
	fmt.Printf("[%0o]\n", uint(1234567))
	fmt.Printf("[%#o]\n", uint(1234567))
	fmt.Printf("[%8o]\n", uint(1234567))
	fmt.Printf("[%-8o]\n", uint(1234567))
	fmt.Printf("[%08o]\n", uint(1234567))
	fmt.Printf("[%+8o]\n", uint(1234567))
	fmt.Printf("[%+08o]\n", uint(1234567))
	fmt.Printf("[% 8o]\n", uint(1234567))
	fmt.Printf("[%8.3o]\n", uint(1234567))
	fmt.Printf("[%.3o]\n", uint(1234567))
	fmt.Printf("[%.0o]\n", uint(1234567))
	fmt.Printf("[%#8o]\n", uint(1234567))
	fmt.Printf("[%#08o]\n", uint(1234567))
	fmt.Printf("[%-#8o]\n", uint(1234567))
	fmt.Printf("[%+.5o]\n", uint(1234567))
	fmt.Printf("[%0-8o]\n", uint(1234567))
	fmt.Printf("[%12.8o]\n", uint(1234567))
	fmt.Printf("[%-12.8o]\n", uint(1234567))
	fmt.Printf("[%#.4o]\n", uint(1234567))
	fmt.Printf("[%+#o]\n", uint(1234567))
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
// [01]
// [       1]
// [1       ]
// [00000001]
// [      +1]
// [+0000001]
// [       1]
// [     001]
// [001]
// [1]
// [      01]
// [00000001]
// [01      ]
// [+00001]
// [1       ]
// [    00000001]
// [00000001    ]
// [0001]
// [+01]
// [52]
// [+52]
// [52]
// [ 52]
// [52]
// [052]
// [      52]
// [52      ]
// [00000052]
// [     +52]
// [+0000052]
// [      52]
// [     052]
// [052]
// [52]
// [     052]
// [00000052]
// [052     ]
// [+00052]
// [52      ]
// [    00000052]
// [00000052    ]
// [0052]
// [+052]
// [4553207]
// [+4553207]
// [4553207]
// [ 4553207]
// [4553207]
// [04553207]
// [ 4553207]
// [4553207 ]
// [04553207]
// [+4553207]
// [+4553207]
// [ 4553207]
// [ 4553207]
// [4553207]
// [4553207]
// [04553207]
// [04553207]
// [04553207]
// [+4553207]
// [4553207 ]
// [    04553207]
// [04553207    ]
// [04553207]
// [+04553207]
