// phase: 05

// Go's %v of integers: code points valid and invalid (surrogates, beyond U+10FFFF, negative),
// with flags.
package main

import "fmt"

func main() {
	fmt.Printf("[%v]\n", 0)
	fmt.Printf("[%+v]\n", 0)
	fmt.Printf("[%-v]\n", 0)
	fmt.Printf("[% v]\n", 0)
	fmt.Printf("[%0v]\n", 0)
	fmt.Printf("[%#v]\n", 0)
	fmt.Printf("[%8v]\n", 0)
	fmt.Printf("[%-8v]\n", 0)
	fmt.Printf("[%08v]\n", 0)
	fmt.Printf("[%+8v]\n", 0)
	fmt.Printf("[%+08v]\n", 0)
	fmt.Printf("[% 8v]\n", 0)
	fmt.Printf("[%8.3v]\n", 0)
	fmt.Printf("[%.3v]\n", 0)
	fmt.Printf("[%.0v]\n", 0)
	fmt.Printf("[%#8v]\n", 0)
	fmt.Printf("[%#08v]\n", 0)
	fmt.Printf("[%-#8v]\n", 0)
	fmt.Printf("[%+.5v]\n", 0)
	fmt.Printf("[%0-8v]\n", 0)
	fmt.Printf("[%12.8v]\n", 0)
	fmt.Printf("[%-12.8v]\n", 0)
	fmt.Printf("[%#.4v]\n", 0)
	fmt.Printf("[%+#v]\n", 0)
	fmt.Printf("[%v]\n", 42)
	fmt.Printf("[%+v]\n", 42)
	fmt.Printf("[%-v]\n", 42)
	fmt.Printf("[% v]\n", 42)
	fmt.Printf("[%0v]\n", 42)
	fmt.Printf("[%#v]\n", 42)
	fmt.Printf("[%8v]\n", 42)
	fmt.Printf("[%-8v]\n", 42)
	fmt.Printf("[%08v]\n", 42)
	fmt.Printf("[%+8v]\n", 42)
	fmt.Printf("[%+08v]\n", 42)
	fmt.Printf("[% 8v]\n", 42)
	fmt.Printf("[%8.3v]\n", 42)
	fmt.Printf("[%.3v]\n", 42)
	fmt.Printf("[%.0v]\n", 42)
	fmt.Printf("[%#8v]\n", 42)
	fmt.Printf("[%#08v]\n", 42)
	fmt.Printf("[%-#8v]\n", 42)
	fmt.Printf("[%+.5v]\n", 42)
	fmt.Printf("[%0-8v]\n", 42)
	fmt.Printf("[%12.8v]\n", 42)
	fmt.Printf("[%-12.8v]\n", 42)
	fmt.Printf("[%#.4v]\n", 42)
	fmt.Printf("[%+#v]\n", 42)
	fmt.Printf("[%v]\n", -42)
	fmt.Printf("[%+v]\n", -42)
	fmt.Printf("[%-v]\n", -42)
	fmt.Printf("[% v]\n", -42)
	fmt.Printf("[%0v]\n", -42)
	fmt.Printf("[%#v]\n", -42)
	fmt.Printf("[%8v]\n", -42)
	fmt.Printf("[%-8v]\n", -42)
	fmt.Printf("[%08v]\n", -42)
	fmt.Printf("[%+8v]\n", -42)
	fmt.Printf("[%+08v]\n", -42)
	fmt.Printf("[% 8v]\n", -42)
	fmt.Printf("[%8.3v]\n", -42)
	fmt.Printf("[%.3v]\n", -42)
	fmt.Printf("[%.0v]\n", -42)
	fmt.Printf("[%#8v]\n", -42)
	fmt.Printf("[%#08v]\n", -42)
	fmt.Printf("[%-#8v]\n", -42)
	fmt.Printf("[%+.5v]\n", -42)
	fmt.Printf("[%0-8v]\n", -42)
	fmt.Printf("[%12.8v]\n", -42)
	fmt.Printf("[%-12.8v]\n", -42)
	fmt.Printf("[%#.4v]\n", -42)
	fmt.Printf("[%+#v]\n", -42)
	fmt.Printf("[%v]\n", int8(-7))
	fmt.Printf("[%+v]\n", int8(-7))
	fmt.Printf("[%-v]\n", int8(-7))
	fmt.Printf("[% v]\n", int8(-7))
	fmt.Printf("[%0v]\n", int8(-7))
	fmt.Printf("[%#v]\n", int8(-7))
	fmt.Printf("[%8v]\n", int8(-7))
	fmt.Printf("[%-8v]\n", int8(-7))
	fmt.Printf("[%08v]\n", int8(-7))
	fmt.Printf("[%+8v]\n", int8(-7))
	fmt.Printf("[%+08v]\n", int8(-7))
	fmt.Printf("[% 8v]\n", int8(-7))
	fmt.Printf("[%8.3v]\n", int8(-7))
	fmt.Printf("[%.3v]\n", int8(-7))
	fmt.Printf("[%.0v]\n", int8(-7))
	fmt.Printf("[%#8v]\n", int8(-7))
	fmt.Printf("[%#08v]\n", int8(-7))
	fmt.Printf("[%-#8v]\n", int8(-7))
	fmt.Printf("[%+.5v]\n", int8(-7))
	fmt.Printf("[%0-8v]\n", int8(-7))
	fmt.Printf("[%12.8v]\n", int8(-7))
	fmt.Printf("[%-12.8v]\n", int8(-7))
	fmt.Printf("[%#.4v]\n", int8(-7))
	fmt.Printf("[%+#v]\n", int8(-7))
	fmt.Printf("[%v]\n", int64(1)<<62)
	fmt.Printf("[%+v]\n", int64(1)<<62)
	fmt.Printf("[%-v]\n", int64(1)<<62)
	fmt.Printf("[% v]\n", int64(1)<<62)
	fmt.Printf("[%0v]\n", int64(1)<<62)
	fmt.Printf("[%#v]\n", int64(1)<<62)
	fmt.Printf("[%8v]\n", int64(1)<<62)
	fmt.Printf("[%-8v]\n", int64(1)<<62)
	fmt.Printf("[%08v]\n", int64(1)<<62)
	fmt.Printf("[%+8v]\n", int64(1)<<62)
	fmt.Printf("[%+08v]\n", int64(1)<<62)
	fmt.Printf("[% 8v]\n", int64(1)<<62)
	fmt.Printf("[%8.3v]\n", int64(1)<<62)
	fmt.Printf("[%.3v]\n", int64(1)<<62)
	fmt.Printf("[%.0v]\n", int64(1)<<62)
	fmt.Printf("[%#8v]\n", int64(1)<<62)
	fmt.Printf("[%#08v]\n", int64(1)<<62)
	fmt.Printf("[%-#8v]\n", int64(1)<<62)
	fmt.Printf("[%+.5v]\n", int64(1)<<62)
	fmt.Printf("[%0-8v]\n", int64(1)<<62)
	fmt.Printf("[%12.8v]\n", int64(1)<<62)
	fmt.Printf("[%-12.8v]\n", int64(1)<<62)
	fmt.Printf("[%#.4v]\n", int64(1)<<62)
	fmt.Printf("[%+#v]\n", int64(1)<<62)
}

// Output:
// [0]
// [0]
// [0]
// [ 0]
// [0]
// [0]
// [       0]
// [0       ]
// [00000000]
// [       0]
// [00000000]
// [       0]
// [     000]
// [000]
// []
// [       0]
// [00000000]
// [0       ]
// [00000]
// [0       ]
// [    00000000]
// [00000000    ]
// [0000]
// [0]
// [42]
// [42]
// [42]
// [ 42]
// [42]
// [42]
// [      42]
// [42      ]
// [00000042]
// [      42]
// [00000042]
// [      42]
// [     042]
// [042]
// [42]
// [      42]
// [00000042]
// [42      ]
// [00042]
// [42      ]
// [    00000042]
// [00000042    ]
// [0042]
// [42]
// [-42]
// [-42]
// [-42]
// [-42]
// [-42]
// [-42]
// [     -42]
// [-42     ]
// [-0000042]
// [     -42]
// [-0000042]
// [     -42]
// [    -042]
// [-042]
// [-42]
// [     -42]
// [-0000042]
// [-42     ]
// [-00042]
// [-42     ]
// [   -00000042]
// [-00000042   ]
// [-0042]
// [-42]
// [-7]
// [-7]
// [-7]
// [-7]
// [-7]
// [-7]
// [      -7]
// [-7      ]
// [-0000007]
// [      -7]
// [-0000007]
// [      -7]
// [    -007]
// [-007]
// [-7]
// [      -7]
// [-0000007]
// [-7      ]
// [-00007]
// [-7      ]
// [   -00000007]
// [-00000007   ]
// [-0007]
// [-7]
// [4611686018427387904]
// [4611686018427387904]
// [4611686018427387904]
// [ 4611686018427387904]
// [4611686018427387904]
// [4611686018427387904]
// [4611686018427387904]
// [4611686018427387904]
// [4611686018427387904]
// [4611686018427387904]
// [4611686018427387904]
// [ 4611686018427387904]
// [4611686018427387904]
// [4611686018427387904]
// [4611686018427387904]
// [4611686018427387904]
// [4611686018427387904]
// [4611686018427387904]
// [4611686018427387904]
// [4611686018427387904]
// [4611686018427387904]
// [4611686018427387904]
// [4611686018427387904]
// [4611686018427387904]
