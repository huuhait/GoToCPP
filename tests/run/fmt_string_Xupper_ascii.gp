// phase: 05

// Go's %X of strings (ascii) with flags, width and precision (runes, not bytes).
package main

import "fmt"

func main() {
	fmt.Printf("[%X]\n", "")
	fmt.Printf("[%+X]\n", "")
	fmt.Printf("[%-X]\n", "")
	fmt.Printf("[% X]\n", "")
	fmt.Printf("[%#X]\n", "")
	fmt.Printf("[%# X]\n", "")
	fmt.Printf("[%10X]\n", "")
	fmt.Printf("[%-10X]\n", "")
	fmt.Printf("[%.3X]\n", "")
	fmt.Printf("[%.0X]\n", "")
	fmt.Printf("[%20.2X]\n", "")
	fmt.Printf("[% 12X]\n", "")
	fmt.Printf("[%#12X]\n", "")
	fmt.Printf("[%020X]\n", "")
	fmt.Printf("[%X]\n", "hello")
	fmt.Printf("[%+X]\n", "hello")
	fmt.Printf("[%-X]\n", "hello")
	fmt.Printf("[% X]\n", "hello")
	fmt.Printf("[%#X]\n", "hello")
	fmt.Printf("[%# X]\n", "hello")
	fmt.Printf("[%10X]\n", "hello")
	fmt.Printf("[%-10X]\n", "hello")
	fmt.Printf("[%.3X]\n", "hello")
	fmt.Printf("[%.0X]\n", "hello")
	fmt.Printf("[%20.2X]\n", "hello")
	fmt.Printf("[% 12X]\n", "hello")
	fmt.Printf("[%#12X]\n", "hello")
	fmt.Printf("[%020X]\n", "hello")
	fmt.Printf("[%X]\n", "hello, world")
	fmt.Printf("[%+X]\n", "hello, world")
	fmt.Printf("[%-X]\n", "hello, world")
	fmt.Printf("[% X]\n", "hello, world")
	fmt.Printf("[%#X]\n", "hello, world")
	fmt.Printf("[%# X]\n", "hello, world")
	fmt.Printf("[%10X]\n", "hello, world")
	fmt.Printf("[%-10X]\n", "hello, world")
	fmt.Printf("[%.3X]\n", "hello, world")
	fmt.Printf("[%.0X]\n", "hello, world")
	fmt.Printf("[%20.2X]\n", "hello, world")
	fmt.Printf("[% 12X]\n", "hello, world")
	fmt.Printf("[%#12X]\n", "hello, world")
	fmt.Printf("[%020X]\n", "hello, world")
	fmt.Printf("[%X]\n", "a b\tc")
	fmt.Printf("[%+X]\n", "a b\tc")
	fmt.Printf("[%-X]\n", "a b\tc")
	fmt.Printf("[% X]\n", "a b\tc")
	fmt.Printf("[%#X]\n", "a b\tc")
	fmt.Printf("[%# X]\n", "a b\tc")
	fmt.Printf("[%10X]\n", "a b\tc")
	fmt.Printf("[%-10X]\n", "a b\tc")
	fmt.Printf("[%.3X]\n", "a b\tc")
	fmt.Printf("[%.0X]\n", "a b\tc")
	fmt.Printf("[%20.2X]\n", "a b\tc")
	fmt.Printf("[% 12X]\n", "a b\tc")
	fmt.Printf("[%#12X]\n", "a b\tc")
	fmt.Printf("[%020X]\n", "a b\tc")
	fmt.Printf("[%X]\n", "line1\nline2")
	fmt.Printf("[%+X]\n", "line1\nline2")
	fmt.Printf("[%-X]\n", "line1\nline2")
	fmt.Printf("[% X]\n", "line1\nline2")
	fmt.Printf("[%#X]\n", "line1\nline2")
	fmt.Printf("[%# X]\n", "line1\nline2")
	fmt.Printf("[%10X]\n", "line1\nline2")
	fmt.Printf("[%-10X]\n", "line1\nline2")
	fmt.Printf("[%.3X]\n", "line1\nline2")
	fmt.Printf("[%.0X]\n", "line1\nline2")
	fmt.Printf("[%20.2X]\n", "line1\nline2")
	fmt.Printf("[% 12X]\n", "line1\nline2")
	fmt.Printf("[%#12X]\n", "line1\nline2")
	fmt.Printf("[%020X]\n", "line1\nline2")
	fmt.Printf("[%X]\n", "quote\"d")
	fmt.Printf("[%+X]\n", "quote\"d")
	fmt.Printf("[%-X]\n", "quote\"d")
	fmt.Printf("[% X]\n", "quote\"d")
	fmt.Printf("[%#X]\n", "quote\"d")
	fmt.Printf("[%# X]\n", "quote\"d")
	fmt.Printf("[%10X]\n", "quote\"d")
	fmt.Printf("[%-10X]\n", "quote\"d")
	fmt.Printf("[%.3X]\n", "quote\"d")
	fmt.Printf("[%.0X]\n", "quote\"d")
	fmt.Printf("[%20.2X]\n", "quote\"d")
	fmt.Printf("[% 12X]\n", "quote\"d")
	fmt.Printf("[%#12X]\n", "quote\"d")
	fmt.Printf("[%020X]\n", "quote\"d")
	fmt.Printf("[%X]\n", "back\\slash")
	fmt.Printf("[%+X]\n", "back\\slash")
	fmt.Printf("[%-X]\n", "back\\slash")
	fmt.Printf("[% X]\n", "back\\slash")
	fmt.Printf("[%#X]\n", "back\\slash")
	fmt.Printf("[%# X]\n", "back\\slash")
	fmt.Printf("[%10X]\n", "back\\slash")
	fmt.Printf("[%-10X]\n", "back\\slash")
	fmt.Printf("[%.3X]\n", "back\\slash")
	fmt.Printf("[%.0X]\n", "back\\slash")
	fmt.Printf("[%20.2X]\n", "back\\slash")
	fmt.Printf("[% 12X]\n", "back\\slash")
	fmt.Printf("[%#12X]\n", "back\\slash")
	fmt.Printf("[%020X]\n", "back\\slash")
	fmt.Printf("[%X]\n", "`tick`")
	fmt.Printf("[%+X]\n", "`tick`")
	fmt.Printf("[%-X]\n", "`tick`")
	fmt.Printf("[% X]\n", "`tick`")
	fmt.Printf("[%#X]\n", "`tick`")
	fmt.Printf("[%# X]\n", "`tick`")
	fmt.Printf("[%10X]\n", "`tick`")
	fmt.Printf("[%-10X]\n", "`tick`")
	fmt.Printf("[%.3X]\n", "`tick`")
	fmt.Printf("[%.0X]\n", "`tick`")
	fmt.Printf("[%20.2X]\n", "`tick`")
	fmt.Printf("[% 12X]\n", "`tick`")
	fmt.Printf("[%#12X]\n", "`tick`")
	fmt.Printf("[%020X]\n", "`tick`")
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
// [68656C6C6F]
// [68656C6C6F]
// [68656C6C6F]
// [68 65 6C 6C 6F]
// [0X68656C6C6F]
// [0X68 0X65 0X6C 0X6C 0X6F]
// [68656C6C6F]
// [68656C6C6F]
// [68656C]
// []
// [                6865]
// [68 65 6C 6C 6F]
// [0X68656C6C6F]
// [000000000068656C6C6F]
// [68656C6C6F2C20776F726C64]
// [68656C6C6F2C20776F726C64]
// [68656C6C6F2C20776F726C64]
// [68 65 6C 6C 6F 2C 20 77 6F 72 6C 64]
// [0X68656C6C6F2C20776F726C64]
// [0X68 0X65 0X6C 0X6C 0X6F 0X2C 0X20 0X77 0X6F 0X72 0X6C 0X64]
// [68656C6C6F2C20776F726C64]
// [68656C6C6F2C20776F726C64]
// [68656C]
// []
// [                6865]
// [68 65 6C 6C 6F 2C 20 77 6F 72 6C 64]
// [0X68656C6C6F2C20776F726C64]
// [68656C6C6F2C20776F726C64]
// [6120620963]
// [6120620963]
// [6120620963]
// [61 20 62 09 63]
// [0X6120620963]
// [0X61 0X20 0X62 0X09 0X63]
// [6120620963]
// [6120620963]
// [612062]
// []
// [                6120]
// [61 20 62 09 63]
// [0X6120620963]
// [00000000006120620963]
// [6C696E65310A6C696E6532]
// [6C696E65310A6C696E6532]
// [6C696E65310A6C696E6532]
// [6C 69 6E 65 31 0A 6C 69 6E 65 32]
// [0X6C696E65310A6C696E6532]
// [0X6C 0X69 0X6E 0X65 0X31 0X0A 0X6C 0X69 0X6E 0X65 0X32]
// [6C696E65310A6C696E6532]
// [6C696E65310A6C696E6532]
// [6C696E]
// []
// [                6C69]
// [6C 69 6E 65 31 0A 6C 69 6E 65 32]
// [0X6C696E65310A6C696E6532]
// [6C696E65310A6C696E6532]
// [71756F74652264]
// [71756F74652264]
// [71756F74652264]
// [71 75 6F 74 65 22 64]
// [0X71756F74652264]
// [0X71 0X75 0X6F 0X74 0X65 0X22 0X64]
// [71756F74652264]
// [71756F74652264]
// [71756F]
// []
// [                7175]
// [71 75 6F 74 65 22 64]
// [0X71756F74652264]
// [00000071756F74652264]
// [6261636B5C736C617368]
// [6261636B5C736C617368]
// [6261636B5C736C617368]
// [62 61 63 6B 5C 73 6C 61 73 68]
// [0X6261636B5C736C617368]
// [0X62 0X61 0X63 0X6B 0X5C 0X73 0X6C 0X61 0X73 0X68]
// [6261636B5C736C617368]
// [6261636B5C736C617368]
// [626163]
// []
// [                6261]
// [62 61 63 6B 5C 73 6C 61 73 68]
// [0X6261636B5C736C617368]
// [6261636B5C736C617368]
// [607469636B60]
// [607469636B60]
// [607469636B60]
// [60 74 69 63 6B 60]
// [0X607469636B60]
// [0X60 0X74 0X69 0X63 0X6B 0X60]
// [607469636B60]
// [607469636B60]
// [607469]
// []
// [                6074]
// [60 74 69 63 6B 60]
// [0X607469636B60]
// [00000000607469636B60]
