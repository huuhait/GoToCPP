// phase: 05

// Go's %d of int16 values with every flag, width and precision: the integer formatting of
// fmt/format.go.
package main

import "fmt"

func main() {
	fmt.Printf("[%d]\n", int16(32767))
	fmt.Printf("[%+d]\n", int16(32767))
	fmt.Printf("[%-d]\n", int16(32767))
	fmt.Printf("[% d]\n", int16(32767))
	fmt.Printf("[%0d]\n", int16(32767))
	fmt.Printf("[%#d]\n", int16(32767))
	fmt.Printf("[%8d]\n", int16(32767))
	fmt.Printf("[%-8d]\n", int16(32767))
	fmt.Printf("[%08d]\n", int16(32767))
	fmt.Printf("[%+8d]\n", int16(32767))
	fmt.Printf("[%+08d]\n", int16(32767))
	fmt.Printf("[% 8d]\n", int16(32767))
	fmt.Printf("[%8.3d]\n", int16(32767))
	fmt.Printf("[%.3d]\n", int16(32767))
	fmt.Printf("[%.0d]\n", int16(32767))
	fmt.Printf("[%#8d]\n", int16(32767))
	fmt.Printf("[%#08d]\n", int16(32767))
	fmt.Printf("[%-#8d]\n", int16(32767))
	fmt.Printf("[%+.5d]\n", int16(32767))
	fmt.Printf("[%0-8d]\n", int16(32767))
	fmt.Printf("[%12.8d]\n", int16(32767))
	fmt.Printf("[%-12.8d]\n", int16(32767))
	fmt.Printf("[%#.4d]\n", int16(32767))
	fmt.Printf("[%+#d]\n", int16(32767))
	fmt.Printf("[%d]\n", int16(-32768))
	fmt.Printf("[%+d]\n", int16(-32768))
	fmt.Printf("[%-d]\n", int16(-32768))
	fmt.Printf("[% d]\n", int16(-32768))
	fmt.Printf("[%0d]\n", int16(-32768))
	fmt.Printf("[%#d]\n", int16(-32768))
	fmt.Printf("[%8d]\n", int16(-32768))
	fmt.Printf("[%-8d]\n", int16(-32768))
	fmt.Printf("[%08d]\n", int16(-32768))
	fmt.Printf("[%+8d]\n", int16(-32768))
	fmt.Printf("[%+08d]\n", int16(-32768))
	fmt.Printf("[% 8d]\n", int16(-32768))
	fmt.Printf("[%8.3d]\n", int16(-32768))
	fmt.Printf("[%.3d]\n", int16(-32768))
	fmt.Printf("[%.0d]\n", int16(-32768))
	fmt.Printf("[%#8d]\n", int16(-32768))
	fmt.Printf("[%#08d]\n", int16(-32768))
	fmt.Printf("[%-#8d]\n", int16(-32768))
	fmt.Printf("[%+.5d]\n", int16(-32768))
	fmt.Printf("[%0-8d]\n", int16(-32768))
	fmt.Printf("[%12.8d]\n", int16(-32768))
	fmt.Printf("[%-12.8d]\n", int16(-32768))
	fmt.Printf("[%#.4d]\n", int16(-32768))
	fmt.Printf("[%+#d]\n", int16(-32768))
	fmt.Printf("[%d]\n", int16(300))
	fmt.Printf("[%+d]\n", int16(300))
	fmt.Printf("[%-d]\n", int16(300))
	fmt.Printf("[% d]\n", int16(300))
	fmt.Printf("[%0d]\n", int16(300))
	fmt.Printf("[%#d]\n", int16(300))
	fmt.Printf("[%8d]\n", int16(300))
	fmt.Printf("[%-8d]\n", int16(300))
	fmt.Printf("[%08d]\n", int16(300))
	fmt.Printf("[%+8d]\n", int16(300))
	fmt.Printf("[%+08d]\n", int16(300))
	fmt.Printf("[% 8d]\n", int16(300))
	fmt.Printf("[%8.3d]\n", int16(300))
	fmt.Printf("[%.3d]\n", int16(300))
	fmt.Printf("[%.0d]\n", int16(300))
	fmt.Printf("[%#8d]\n", int16(300))
	fmt.Printf("[%#08d]\n", int16(300))
	fmt.Printf("[%-#8d]\n", int16(300))
	fmt.Printf("[%+.5d]\n", int16(300))
	fmt.Printf("[%0-8d]\n", int16(300))
	fmt.Printf("[%12.8d]\n", int16(300))
	fmt.Printf("[%-12.8d]\n", int16(300))
	fmt.Printf("[%#.4d]\n", int16(300))
	fmt.Printf("[%+#d]\n", int16(300))
}

// Output:
// [32767]
// [+32767]
// [32767]
// [ 32767]
// [32767]
// [32767]
// [   32767]
// [32767   ]
// [00032767]
// [  +32767]
// [+0032767]
// [   32767]
// [   32767]
// [32767]
// [32767]
// [   32767]
// [00032767]
// [32767   ]
// [+32767]
// [32767   ]
// [    00032767]
// [00032767    ]
// [32767]
// [+32767]
// [-32768]
// [-32768]
// [-32768]
// [-32768]
// [-32768]
// [-32768]
// [  -32768]
// [-32768  ]
// [-0032768]
// [  -32768]
// [-0032768]
// [  -32768]
// [  -32768]
// [-32768]
// [-32768]
// [  -32768]
// [-0032768]
// [-32768  ]
// [-32768]
// [-32768  ]
// [   -00032768]
// [-00032768   ]
// [-32768]
// [-32768]
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
