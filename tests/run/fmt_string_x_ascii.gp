// phase: 05

// Go's %x of strings (ascii) with flags, width and precision (runes, not bytes).
package main

import "fmt"

func main() {
	fmt.Printf("[%x]\n", "")
	fmt.Printf("[%+x]\n", "")
	fmt.Printf("[%-x]\n", "")
	fmt.Printf("[% x]\n", "")
	fmt.Printf("[%#x]\n", "")
	fmt.Printf("[%# x]\n", "")
	fmt.Printf("[%10x]\n", "")
	fmt.Printf("[%-10x]\n", "")
	fmt.Printf("[%.3x]\n", "")
	fmt.Printf("[%.0x]\n", "")
	fmt.Printf("[%20.2x]\n", "")
	fmt.Printf("[% 12x]\n", "")
	fmt.Printf("[%#12x]\n", "")
	fmt.Printf("[%020x]\n", "")
	fmt.Printf("[%x]\n", "hello")
	fmt.Printf("[%+x]\n", "hello")
	fmt.Printf("[%-x]\n", "hello")
	fmt.Printf("[% x]\n", "hello")
	fmt.Printf("[%#x]\n", "hello")
	fmt.Printf("[%# x]\n", "hello")
	fmt.Printf("[%10x]\n", "hello")
	fmt.Printf("[%-10x]\n", "hello")
	fmt.Printf("[%.3x]\n", "hello")
	fmt.Printf("[%.0x]\n", "hello")
	fmt.Printf("[%20.2x]\n", "hello")
	fmt.Printf("[% 12x]\n", "hello")
	fmt.Printf("[%#12x]\n", "hello")
	fmt.Printf("[%020x]\n", "hello")
	fmt.Printf("[%x]\n", "hello, world")
	fmt.Printf("[%+x]\n", "hello, world")
	fmt.Printf("[%-x]\n", "hello, world")
	fmt.Printf("[% x]\n", "hello, world")
	fmt.Printf("[%#x]\n", "hello, world")
	fmt.Printf("[%# x]\n", "hello, world")
	fmt.Printf("[%10x]\n", "hello, world")
	fmt.Printf("[%-10x]\n", "hello, world")
	fmt.Printf("[%.3x]\n", "hello, world")
	fmt.Printf("[%.0x]\n", "hello, world")
	fmt.Printf("[%20.2x]\n", "hello, world")
	fmt.Printf("[% 12x]\n", "hello, world")
	fmt.Printf("[%#12x]\n", "hello, world")
	fmt.Printf("[%020x]\n", "hello, world")
	fmt.Printf("[%x]\n", "a b\tc")
	fmt.Printf("[%+x]\n", "a b\tc")
	fmt.Printf("[%-x]\n", "a b\tc")
	fmt.Printf("[% x]\n", "a b\tc")
	fmt.Printf("[%#x]\n", "a b\tc")
	fmt.Printf("[%# x]\n", "a b\tc")
	fmt.Printf("[%10x]\n", "a b\tc")
	fmt.Printf("[%-10x]\n", "a b\tc")
	fmt.Printf("[%.3x]\n", "a b\tc")
	fmt.Printf("[%.0x]\n", "a b\tc")
	fmt.Printf("[%20.2x]\n", "a b\tc")
	fmt.Printf("[% 12x]\n", "a b\tc")
	fmt.Printf("[%#12x]\n", "a b\tc")
	fmt.Printf("[%020x]\n", "a b\tc")
	fmt.Printf("[%x]\n", "line1\nline2")
	fmt.Printf("[%+x]\n", "line1\nline2")
	fmt.Printf("[%-x]\n", "line1\nline2")
	fmt.Printf("[% x]\n", "line1\nline2")
	fmt.Printf("[%#x]\n", "line1\nline2")
	fmt.Printf("[%# x]\n", "line1\nline2")
	fmt.Printf("[%10x]\n", "line1\nline2")
	fmt.Printf("[%-10x]\n", "line1\nline2")
	fmt.Printf("[%.3x]\n", "line1\nline2")
	fmt.Printf("[%.0x]\n", "line1\nline2")
	fmt.Printf("[%20.2x]\n", "line1\nline2")
	fmt.Printf("[% 12x]\n", "line1\nline2")
	fmt.Printf("[%#12x]\n", "line1\nline2")
	fmt.Printf("[%020x]\n", "line1\nline2")
	fmt.Printf("[%x]\n", "quote\"d")
	fmt.Printf("[%+x]\n", "quote\"d")
	fmt.Printf("[%-x]\n", "quote\"d")
	fmt.Printf("[% x]\n", "quote\"d")
	fmt.Printf("[%#x]\n", "quote\"d")
	fmt.Printf("[%# x]\n", "quote\"d")
	fmt.Printf("[%10x]\n", "quote\"d")
	fmt.Printf("[%-10x]\n", "quote\"d")
	fmt.Printf("[%.3x]\n", "quote\"d")
	fmt.Printf("[%.0x]\n", "quote\"d")
	fmt.Printf("[%20.2x]\n", "quote\"d")
	fmt.Printf("[% 12x]\n", "quote\"d")
	fmt.Printf("[%#12x]\n", "quote\"d")
	fmt.Printf("[%020x]\n", "quote\"d")
	fmt.Printf("[%x]\n", "back\\slash")
	fmt.Printf("[%+x]\n", "back\\slash")
	fmt.Printf("[%-x]\n", "back\\slash")
	fmt.Printf("[% x]\n", "back\\slash")
	fmt.Printf("[%#x]\n", "back\\slash")
	fmt.Printf("[%# x]\n", "back\\slash")
	fmt.Printf("[%10x]\n", "back\\slash")
	fmt.Printf("[%-10x]\n", "back\\slash")
	fmt.Printf("[%.3x]\n", "back\\slash")
	fmt.Printf("[%.0x]\n", "back\\slash")
	fmt.Printf("[%20.2x]\n", "back\\slash")
	fmt.Printf("[% 12x]\n", "back\\slash")
	fmt.Printf("[%#12x]\n", "back\\slash")
	fmt.Printf("[%020x]\n", "back\\slash")
	fmt.Printf("[%x]\n", "`tick`")
	fmt.Printf("[%+x]\n", "`tick`")
	fmt.Printf("[%-x]\n", "`tick`")
	fmt.Printf("[% x]\n", "`tick`")
	fmt.Printf("[%#x]\n", "`tick`")
	fmt.Printf("[%# x]\n", "`tick`")
	fmt.Printf("[%10x]\n", "`tick`")
	fmt.Printf("[%-10x]\n", "`tick`")
	fmt.Printf("[%.3x]\n", "`tick`")
	fmt.Printf("[%.0x]\n", "`tick`")
	fmt.Printf("[%20.2x]\n", "`tick`")
	fmt.Printf("[% 12x]\n", "`tick`")
	fmt.Printf("[%#12x]\n", "`tick`")
	fmt.Printf("[%020x]\n", "`tick`")
}

// Output:
// []
// []
// []
// []
// []
// []
// [          ]
// [          ]
// []
// []
// [                    ]
// [            ]
// [            ]
// [00000000000000000000]
// [68656c6c6f]
// [68656c6c6f]
// [68656c6c6f]
// [68 65 6c 6c 6f]
// [0x68656c6c6f]
// [0x68 0x65 0x6c 0x6c 0x6f]
// [68656c6c6f]
// [68656c6c6f]
// [68656c]
// []
// [                6865]
// [68 65 6c 6c 6f]
// [0x68656c6c6f]
// [000000000068656c6c6f]
// [68656c6c6f2c20776f726c64]
// [68656c6c6f2c20776f726c64]
// [68656c6c6f2c20776f726c64]
// [68 65 6c 6c 6f 2c 20 77 6f 72 6c 64]
// [0x68656c6c6f2c20776f726c64]
// [0x68 0x65 0x6c 0x6c 0x6f 0x2c 0x20 0x77 0x6f 0x72 0x6c 0x64]
// [68656c6c6f2c20776f726c64]
// [68656c6c6f2c20776f726c64]
// [68656c]
// []
// [                6865]
// [68 65 6c 6c 6f 2c 20 77 6f 72 6c 64]
// [0x68656c6c6f2c20776f726c64]
// [68656c6c6f2c20776f726c64]
// [6120620963]
// [6120620963]
// [6120620963]
// [61 20 62 09 63]
// [0x6120620963]
// [0x61 0x20 0x62 0x09 0x63]
// [6120620963]
// [6120620963]
// [612062]
// []
// [                6120]
// [61 20 62 09 63]
// [0x6120620963]
// [00000000006120620963]
// [6c696e65310a6c696e6532]
// [6c696e65310a6c696e6532]
// [6c696e65310a6c696e6532]
// [6c 69 6e 65 31 0a 6c 69 6e 65 32]
// [0x6c696e65310a6c696e6532]
// [0x6c 0x69 0x6e 0x65 0x31 0x0a 0x6c 0x69 0x6e 0x65 0x32]
// [6c696e65310a6c696e6532]
// [6c696e65310a6c696e6532]
// [6c696e]
// []
// [                6c69]
// [6c 69 6e 65 31 0a 6c 69 6e 65 32]
// [0x6c696e65310a6c696e6532]
// [6c696e65310a6c696e6532]
// [71756f74652264]
// [71756f74652264]
// [71756f74652264]
// [71 75 6f 74 65 22 64]
// [0x71756f74652264]
// [0x71 0x75 0x6f 0x74 0x65 0x22 0x64]
// [71756f74652264]
// [71756f74652264]
// [71756f]
// []
// [                7175]
// [71 75 6f 74 65 22 64]
// [0x71756f74652264]
// [00000071756f74652264]
// [6261636b5c736c617368]
// [6261636b5c736c617368]
// [6261636b5c736c617368]
// [62 61 63 6b 5c 73 6c 61 73 68]
// [0x6261636b5c736c617368]
// [0x62 0x61 0x63 0x6b 0x5c 0x73 0x6c 0x61 0x73 0x68]
// [6261636b5c736c617368]
// [6261636b5c736c617368]
// [626163]
// []
// [                6261]
// [62 61 63 6b 5c 73 6c 61 73 68]
// [0x6261636b5c736c617368]
// [6261636b5c736c617368]
// [607469636b60]
// [607469636b60]
// [607469636b60]
// [60 74 69 63 6b 60]
// [0x607469636b60]
// [0x60 0x74 0x69 0x63 0x6b 0x60]
// [607469636b60]
// [607469636b60]
// [607469]
// []
// [                6074]
// [60 74 69 63 6b 60]
// [0x607469636b60]
// [00000000607469636b60]
