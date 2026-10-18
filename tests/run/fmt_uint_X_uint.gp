// phase: 05

// Go's %X of uint values with flags, width and precision.
package main

import "fmt"

func main() {
	fmt.Printf("[%X]\n", uint(0))
	fmt.Printf("[%+X]\n", uint(0))
	fmt.Printf("[%-X]\n", uint(0))
	fmt.Printf("[% X]\n", uint(0))
	fmt.Printf("[%0X]\n", uint(0))
	fmt.Printf("[%#X]\n", uint(0))
	fmt.Printf("[%8X]\n", uint(0))
	fmt.Printf("[%-8X]\n", uint(0))
	fmt.Printf("[%08X]\n", uint(0))
	fmt.Printf("[%+8X]\n", uint(0))
	fmt.Printf("[%+08X]\n", uint(0))
	fmt.Printf("[% 8X]\n", uint(0))
	fmt.Printf("[%8.3X]\n", uint(0))
	fmt.Printf("[%.3X]\n", uint(0))
	fmt.Printf("[%.0X]\n", uint(0))
	fmt.Printf("[%#8X]\n", uint(0))
	fmt.Printf("[%#08X]\n", uint(0))
	fmt.Printf("[%-#8X]\n", uint(0))
	fmt.Printf("[%+.5X]\n", uint(0))
	fmt.Printf("[%0-8X]\n", uint(0))
	fmt.Printf("[%12.8X]\n", uint(0))
	fmt.Printf("[%-12.8X]\n", uint(0))
	fmt.Printf("[%#.4X]\n", uint(0))
	fmt.Printf("[%+#X]\n", uint(0))
	fmt.Printf("[%X]\n", uint(1))
	fmt.Printf("[%+X]\n", uint(1))
	fmt.Printf("[%-X]\n", uint(1))
	fmt.Printf("[% X]\n", uint(1))
	fmt.Printf("[%0X]\n", uint(1))
	fmt.Printf("[%#X]\n", uint(1))
	fmt.Printf("[%8X]\n", uint(1))
	fmt.Printf("[%-8X]\n", uint(1))
	fmt.Printf("[%08X]\n", uint(1))
	fmt.Printf("[%+8X]\n", uint(1))
	fmt.Printf("[%+08X]\n", uint(1))
	fmt.Printf("[% 8X]\n", uint(1))
	fmt.Printf("[%8.3X]\n", uint(1))
	fmt.Printf("[%.3X]\n", uint(1))
	fmt.Printf("[%.0X]\n", uint(1))
	fmt.Printf("[%#8X]\n", uint(1))
	fmt.Printf("[%#08X]\n", uint(1))
	fmt.Printf("[%-#8X]\n", uint(1))
	fmt.Printf("[%+.5X]\n", uint(1))
	fmt.Printf("[%0-8X]\n", uint(1))
	fmt.Printf("[%12.8X]\n", uint(1))
	fmt.Printf("[%-12.8X]\n", uint(1))
	fmt.Printf("[%#.4X]\n", uint(1))
	fmt.Printf("[%+#X]\n", uint(1))
	fmt.Printf("[%X]\n", uint(42))
	fmt.Printf("[%+X]\n", uint(42))
	fmt.Printf("[%-X]\n", uint(42))
	fmt.Printf("[% X]\n", uint(42))
	fmt.Printf("[%0X]\n", uint(42))
	fmt.Printf("[%#X]\n", uint(42))
	fmt.Printf("[%8X]\n", uint(42))
	fmt.Printf("[%-8X]\n", uint(42))
	fmt.Printf("[%08X]\n", uint(42))
	fmt.Printf("[%+8X]\n", uint(42))
	fmt.Printf("[%+08X]\n", uint(42))
	fmt.Printf("[% 8X]\n", uint(42))
	fmt.Printf("[%8.3X]\n", uint(42))
	fmt.Printf("[%.3X]\n", uint(42))
	fmt.Printf("[%.0X]\n", uint(42))
	fmt.Printf("[%#8X]\n", uint(42))
	fmt.Printf("[%#08X]\n", uint(42))
	fmt.Printf("[%-#8X]\n", uint(42))
	fmt.Printf("[%+.5X]\n", uint(42))
	fmt.Printf("[%0-8X]\n", uint(42))
	fmt.Printf("[%12.8X]\n", uint(42))
	fmt.Printf("[%-12.8X]\n", uint(42))
	fmt.Printf("[%#.4X]\n", uint(42))
	fmt.Printf("[%+#X]\n", uint(42))
	fmt.Printf("[%X]\n", uint(1234567))
	fmt.Printf("[%+X]\n", uint(1234567))
	fmt.Printf("[%-X]\n", uint(1234567))
	fmt.Printf("[% X]\n", uint(1234567))
	fmt.Printf("[%0X]\n", uint(1234567))
	fmt.Printf("[%#X]\n", uint(1234567))
	fmt.Printf("[%8X]\n", uint(1234567))
	fmt.Printf("[%-8X]\n", uint(1234567))
	fmt.Printf("[%08X]\n", uint(1234567))
	fmt.Printf("[%+8X]\n", uint(1234567))
	fmt.Printf("[%+08X]\n", uint(1234567))
	fmt.Printf("[% 8X]\n", uint(1234567))
	fmt.Printf("[%8.3X]\n", uint(1234567))
	fmt.Printf("[%.3X]\n", uint(1234567))
	fmt.Printf("[%.0X]\n", uint(1234567))
	fmt.Printf("[%#8X]\n", uint(1234567))
	fmt.Printf("[%#08X]\n", uint(1234567))
	fmt.Printf("[%-#8X]\n", uint(1234567))
	fmt.Printf("[%+.5X]\n", uint(1234567))
	fmt.Printf("[%0-8X]\n", uint(1234567))
	fmt.Printf("[%12.8X]\n", uint(1234567))
	fmt.Printf("[%-12.8X]\n", uint(1234567))
	fmt.Printf("[%#.4X]\n", uint(1234567))
	fmt.Printf("[%+#X]\n", uint(1234567))
}

// Output:
// [0]
// [+0]
// [0]
// [ 0]
// [0]
// [0X0]
// [       0]
// [0       ]
// [00000000]
// [      +0]
// [+0000000]
// [       0]
// [     000]
// [000]
// []
// [     0X0]
// [0X00000000]
// [0X0     ]
// [+00000]
// [0       ]
// [    00000000]
// [00000000    ]
// [0X0000]
// [+0X0]
// [1]
// [+1]
// [1]
// [ 1]
// [1]
// [0X1]
// [       1]
// [1       ]
// [00000001]
// [      +1]
// [+0000001]
// [       1]
// [     001]
// [001]
// [1]
// [     0X1]
// [0X00000001]
// [0X1     ]
// [+00001]
// [1       ]
// [    00000001]
// [00000001    ]
// [0X0001]
// [+0X1]
// [2A]
// [+2A]
// [2A]
// [ 2A]
// [2A]
// [0X2A]
// [      2A]
// [2A      ]
// [0000002A]
// [     +2A]
// [+000002A]
// [      2A]
// [     02A]
// [02A]
// [2A]
// [    0X2A]
// [0X0000002A]
// [0X2A    ]
// [+0002A]
// [2A      ]
// [    0000002A]
// [0000002A    ]
// [0X002A]
// [+0X2A]
// [12D687]
// [+12D687]
// [12D687]
// [ 12D687]
// [12D687]
// [0X12D687]
// [  12D687]
// [12D687  ]
// [0012D687]
// [ +12D687]
// [+012D687]
// [  12D687]
// [  12D687]
// [12D687]
// [12D687]
// [0X12D687]
// [0X0012D687]
// [0X12D687]
// [+12D687]
// [12D687  ]
// [    0012D687]
// [0012D687    ]
// [0X12D687]
// [+0X12D687]
