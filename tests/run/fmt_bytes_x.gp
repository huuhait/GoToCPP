// phase: 05

// Go's %x of byte slices: %s %q %x %X treat them as strings, %v and %d as lists of numbers.
package main

import "fmt"

func main() {
	fmt.Printf("[%x]\n", []byte(nil))
	fmt.Printf("[%+x]\n", []byte(nil))
	fmt.Printf("[%-x]\n", []byte(nil))
	fmt.Printf("[% x]\n", []byte(nil))
	fmt.Printf("[%#x]\n", []byte(nil))
	fmt.Printf("[%10x]\n", []byte(nil))
	fmt.Printf("[%-10x]\n", []byte(nil))
	fmt.Printf("[%.3x]\n", []byte(nil))
	fmt.Printf("[%10.2x]\n", []byte(nil))
	fmt.Printf("[%# x]\n", []byte(nil))
	fmt.Printf("[%#10x]\n", []byte(nil))
	fmt.Printf("[%020x]\n", []byte(nil))
	fmt.Printf("[%x]\n", []byte{})
	fmt.Printf("[%+x]\n", []byte{})
	fmt.Printf("[%-x]\n", []byte{})
	fmt.Printf("[% x]\n", []byte{})
	fmt.Printf("[%#x]\n", []byte{})
	fmt.Printf("[%10x]\n", []byte{})
	fmt.Printf("[%-10x]\n", []byte{})
	fmt.Printf("[%.3x]\n", []byte{})
	fmt.Printf("[%10.2x]\n", []byte{})
	fmt.Printf("[%# x]\n", []byte{})
	fmt.Printf("[%#10x]\n", []byte{})
	fmt.Printf("[%020x]\n", []byte{})
	fmt.Printf("[%x]\n", []byte("hello"))
	fmt.Printf("[%+x]\n", []byte("hello"))
	fmt.Printf("[%-x]\n", []byte("hello"))
	fmt.Printf("[% x]\n", []byte("hello"))
	fmt.Printf("[%#x]\n", []byte("hello"))
	fmt.Printf("[%10x]\n", []byte("hello"))
	fmt.Printf("[%-10x]\n", []byte("hello"))
	fmt.Printf("[%.3x]\n", []byte("hello"))
	fmt.Printf("[%10.2x]\n", []byte("hello"))
	fmt.Printf("[%# x]\n", []byte("hello"))
	fmt.Printf("[%#10x]\n", []byte("hello"))
	fmt.Printf("[%020x]\n", []byte("hello"))
	fmt.Printf("[%x]\n", []byte{0, 1, 2, 254, 255})
	fmt.Printf("[%+x]\n", []byte{0, 1, 2, 254, 255})
	fmt.Printf("[%-x]\n", []byte{0, 1, 2, 254, 255})
	fmt.Printf("[% x]\n", []byte{0, 1, 2, 254, 255})
	fmt.Printf("[%#x]\n", []byte{0, 1, 2, 254, 255})
	fmt.Printf("[%10x]\n", []byte{0, 1, 2, 254, 255})
	fmt.Printf("[%-10x]\n", []byte{0, 1, 2, 254, 255})
	fmt.Printf("[%.3x]\n", []byte{0, 1, 2, 254, 255})
	fmt.Printf("[%10.2x]\n", []byte{0, 1, 2, 254, 255})
	fmt.Printf("[%# x]\n", []byte{0, 1, 2, 254, 255})
	fmt.Printf("[%#10x]\n", []byte{0, 1, 2, 254, 255})
	fmt.Printf("[%020x]\n", []byte{0, 1, 2, 254, 255})
	fmt.Printf("[%x]\n", []byte("日本語"))
	fmt.Printf("[%+x]\n", []byte("日本語"))
	fmt.Printf("[%-x]\n", []byte("日本語"))
	fmt.Printf("[% x]\n", []byte("日本語"))
	fmt.Printf("[%#x]\n", []byte("日本語"))
	fmt.Printf("[%10x]\n", []byte("日本語"))
	fmt.Printf("[%-10x]\n", []byte("日本語"))
	fmt.Printf("[%.3x]\n", []byte("日本語"))
	fmt.Printf("[%10.2x]\n", []byte("日本語"))
	fmt.Printf("[%# x]\n", []byte("日本語"))
	fmt.Printf("[%#10x]\n", []byte("日本語"))
	fmt.Printf("[%020x]\n", []byte("日本語"))
	fmt.Printf("[%x]\n", []byte("a b"))
	fmt.Printf("[%+x]\n", []byte("a b"))
	fmt.Printf("[%-x]\n", []byte("a b"))
	fmt.Printf("[% x]\n", []byte("a b"))
	fmt.Printf("[%#x]\n", []byte("a b"))
	fmt.Printf("[%10x]\n", []byte("a b"))
	fmt.Printf("[%-10x]\n", []byte("a b"))
	fmt.Printf("[%.3x]\n", []byte("a b"))
	fmt.Printf("[%10.2x]\n", []byte("a b"))
	fmt.Printf("[%# x]\n", []byte("a b"))
	fmt.Printf("[%#10x]\n", []byte("a b"))
	fmt.Printf("[%020x]\n", []byte("a b"))
	fmt.Printf("[%x]\n", []byte("tab\t"))
	fmt.Printf("[%+x]\n", []byte("tab\t"))
	fmt.Printf("[%-x]\n", []byte("tab\t"))
	fmt.Printf("[% x]\n", []byte("tab\t"))
	fmt.Printf("[%#x]\n", []byte("tab\t"))
	fmt.Printf("[%10x]\n", []byte("tab\t"))
	fmt.Printf("[%-10x]\n", []byte("tab\t"))
	fmt.Printf("[%.3x]\n", []byte("tab\t"))
	fmt.Printf("[%10.2x]\n", []byte("tab\t"))
	fmt.Printf("[%# x]\n", []byte("tab\t"))
	fmt.Printf("[%#10x]\n", []byte("tab\t"))
	fmt.Printf("[%020x]\n", []byte("tab\t"))
}

// Output:
// []
// []
// []
// []
// []
// [          ]
// [          ]
// []
// [          ]
// []
// [          ]
// [00000000000000000000]
// []
// []
// []
// []
// []
// [          ]
// [          ]
// []
// [          ]
// []
// [          ]
// [00000000000000000000]
// [68656c6c6f]
// [68656c6c6f]
// [68656c6c6f]
// [68 65 6c 6c 6f]
// [0x68656c6c6f]
// [68656c6c6f]
// [68656c6c6f]
// [68656c]
// [      6865]
// [0x68 0x65 0x6c 0x6c 0x6f]
// [0x68656c6c6f]
// [000000000068656c6c6f]
// [000102feff]
// [000102feff]
// [000102feff]
// [00 01 02 fe ff]
// [0x000102feff]
// [000102feff]
// [000102feff]
// [000102]
// [      0001]
// [0x00 0x01 0x02 0xfe 0xff]
// [0x000102feff]
// [0000000000000102feff]
// [e697a5e69cace8aa9e]
// [e697a5e69cace8aa9e]
// [e697a5e69cace8aa9e]
// [e6 97 a5 e6 9c ac e8 aa 9e]
// [0xe697a5e69cace8aa9e]
// [e697a5e69cace8aa9e]
// [e697a5e69cace8aa9e]
// [e697a5]
// [      e697]
// [0xe6 0x97 0xa5 0xe6 0x9c 0xac 0xe8 0xaa 0x9e]
// [0xe697a5e69cace8aa9e]
// [00e697a5e69cace8aa9e]
// [612062]
// [612062]
// [612062]
// [61 20 62]
// [0x612062]
// [    612062]
// [612062    ]
// [612062]
// [      6120]
// [0x61 0x20 0x62]
// [  0x612062]
// [00000000000000612062]
// [74616209]
// [74616209]
// [74616209]
// [74 61 62 09]
// [0x74616209]
// [  74616209]
// [74616209  ]
// [746162]
// [      7461]
// [0x74 0x61 0x62 0x09]
// [0x74616209]
// [00000000000074616209]
