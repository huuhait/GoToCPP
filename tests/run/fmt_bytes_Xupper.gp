// phase: 05

// Go's %X of byte slices: %s %q %x %X treat them as strings, %v and %d as lists of numbers.
package main

import "fmt"

func main() {
	fmt.Printf("[%X]\n", []byte(nil))
	fmt.Printf("[%+X]\n", []byte(nil))
	fmt.Printf("[%-X]\n", []byte(nil))
	fmt.Printf("[% X]\n", []byte(nil))
	fmt.Printf("[%#X]\n", []byte(nil))
	fmt.Printf("[%10X]\n", []byte(nil))
	fmt.Printf("[%-10X]\n", []byte(nil))
	fmt.Printf("[%.3X]\n", []byte(nil))
	fmt.Printf("[%10.2X]\n", []byte(nil))
	fmt.Printf("[%# X]\n", []byte(nil))
	fmt.Printf("[%#10X]\n", []byte(nil))
	fmt.Printf("[%020X]\n", []byte(nil))
	fmt.Printf("[%X]\n", []byte{})
	fmt.Printf("[%+X]\n", []byte{})
	fmt.Printf("[%-X]\n", []byte{})
	fmt.Printf("[% X]\n", []byte{})
	fmt.Printf("[%#X]\n", []byte{})
	fmt.Printf("[%10X]\n", []byte{})
	fmt.Printf("[%-10X]\n", []byte{})
	fmt.Printf("[%.3X]\n", []byte{})
	fmt.Printf("[%10.2X]\n", []byte{})
	fmt.Printf("[%# X]\n", []byte{})
	fmt.Printf("[%#10X]\n", []byte{})
	fmt.Printf("[%020X]\n", []byte{})
	fmt.Printf("[%X]\n", []byte("hello"))
	fmt.Printf("[%+X]\n", []byte("hello"))
	fmt.Printf("[%-X]\n", []byte("hello"))
	fmt.Printf("[% X]\n", []byte("hello"))
	fmt.Printf("[%#X]\n", []byte("hello"))
	fmt.Printf("[%10X]\n", []byte("hello"))
	fmt.Printf("[%-10X]\n", []byte("hello"))
	fmt.Printf("[%.3X]\n", []byte("hello"))
	fmt.Printf("[%10.2X]\n", []byte("hello"))
	fmt.Printf("[%# X]\n", []byte("hello"))
	fmt.Printf("[%#10X]\n", []byte("hello"))
	fmt.Printf("[%020X]\n", []byte("hello"))
	fmt.Printf("[%X]\n", []byte{0, 1, 2, 254, 255})
	fmt.Printf("[%+X]\n", []byte{0, 1, 2, 254, 255})
	fmt.Printf("[%-X]\n", []byte{0, 1, 2, 254, 255})
	fmt.Printf("[% X]\n", []byte{0, 1, 2, 254, 255})
	fmt.Printf("[%#X]\n", []byte{0, 1, 2, 254, 255})
	fmt.Printf("[%10X]\n", []byte{0, 1, 2, 254, 255})
	fmt.Printf("[%-10X]\n", []byte{0, 1, 2, 254, 255})
	fmt.Printf("[%.3X]\n", []byte{0, 1, 2, 254, 255})
	fmt.Printf("[%10.2X]\n", []byte{0, 1, 2, 254, 255})
	fmt.Printf("[%# X]\n", []byte{0, 1, 2, 254, 255})
	fmt.Printf("[%#10X]\n", []byte{0, 1, 2, 254, 255})
	fmt.Printf("[%020X]\n", []byte{0, 1, 2, 254, 255})
	fmt.Printf("[%X]\n", []byte("日本語"))
	fmt.Printf("[%+X]\n", []byte("日本語"))
	fmt.Printf("[%-X]\n", []byte("日本語"))
	fmt.Printf("[% X]\n", []byte("日本語"))
	fmt.Printf("[%#X]\n", []byte("日本語"))
	fmt.Printf("[%10X]\n", []byte("日本語"))
	fmt.Printf("[%-10X]\n", []byte("日本語"))
	fmt.Printf("[%.3X]\n", []byte("日本語"))
	fmt.Printf("[%10.2X]\n", []byte("日本語"))
	fmt.Printf("[%# X]\n", []byte("日本語"))
	fmt.Printf("[%#10X]\n", []byte("日本語"))
	fmt.Printf("[%020X]\n", []byte("日本語"))
	fmt.Printf("[%X]\n", []byte("a b"))
	fmt.Printf("[%+X]\n", []byte("a b"))
	fmt.Printf("[%-X]\n", []byte("a b"))
	fmt.Printf("[% X]\n", []byte("a b"))
	fmt.Printf("[%#X]\n", []byte("a b"))
	fmt.Printf("[%10X]\n", []byte("a b"))
	fmt.Printf("[%-10X]\n", []byte("a b"))
	fmt.Printf("[%.3X]\n", []byte("a b"))
	fmt.Printf("[%10.2X]\n", []byte("a b"))
	fmt.Printf("[%# X]\n", []byte("a b"))
	fmt.Printf("[%#10X]\n", []byte("a b"))
	fmt.Printf("[%020X]\n", []byte("a b"))
	fmt.Printf("[%X]\n", []byte("tab\t"))
	fmt.Printf("[%+X]\n", []byte("tab\t"))
	fmt.Printf("[%-X]\n", []byte("tab\t"))
	fmt.Printf("[% X]\n", []byte("tab\t"))
	fmt.Printf("[%#X]\n", []byte("tab\t"))
	fmt.Printf("[%10X]\n", []byte("tab\t"))
	fmt.Printf("[%-10X]\n", []byte("tab\t"))
	fmt.Printf("[%.3X]\n", []byte("tab\t"))
	fmt.Printf("[%10.2X]\n", []byte("tab\t"))
	fmt.Printf("[%# X]\n", []byte("tab\t"))
	fmt.Printf("[%#10X]\n", []byte("tab\t"))
	fmt.Printf("[%020X]\n", []byte("tab\t"))
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
// [68656C6C6F]
// [68656C6C6F]
// [68656C6C6F]
// [68 65 6C 6C 6F]
// [0X68656C6C6F]
// [68656C6C6F]
// [68656C6C6F]
// [68656C]
// [      6865]
// [0X68 0X65 0X6C 0X6C 0X6F]
// [0X68656C6C6F]
// [000000000068656C6C6F]
// [000102FEFF]
// [000102FEFF]
// [000102FEFF]
// [00 01 02 FE FF]
// [0X000102FEFF]
// [000102FEFF]
// [000102FEFF]
// [000102]
// [      0001]
// [0X00 0X01 0X02 0XFE 0XFF]
// [0X000102FEFF]
// [0000000000000102FEFF]
// [E697A5E69CACE8AA9E]
// [E697A5E69CACE8AA9E]
// [E697A5E69CACE8AA9E]
// [E6 97 A5 E6 9C AC E8 AA 9E]
// [0XE697A5E69CACE8AA9E]
// [E697A5E69CACE8AA9E]
// [E697A5E69CACE8AA9E]
// [E697A5]
// [      E697]
// [0XE6 0X97 0XA5 0XE6 0X9C 0XAC 0XE8 0XAA 0X9E]
// [0XE697A5E69CACE8AA9E]
// [00E697A5E69CACE8AA9E]
// [612062]
// [612062]
// [612062]
// [61 20 62]
// [0X612062]
// [    612062]
// [612062    ]
// [612062]
// [      6120]
// [0X61 0X20 0X62]
// [  0X612062]
// [00000000000000612062]
// [74616209]
// [74616209]
// [74616209]
// [74 61 62 09]
// [0X74616209]
// [  74616209]
// [74616209  ]
// [746162]
// [      7461]
// [0X74 0X61 0X62 0X09]
// [0X74616209]
// [00000000000074616209]
