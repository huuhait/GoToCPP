// phase: 05

// Go's %X of strings (controls) with flags, width and precision (runes, not bytes).
package main

import "fmt"

func main() {
	fmt.Printf("[%X]\n", "\u2028sep")
	fmt.Printf("[%+X]\n", "\u2028sep")
	fmt.Printf("[%-X]\n", "\u2028sep")
	fmt.Printf("[% X]\n", "\u2028sep")
	fmt.Printf("[%#X]\n", "\u2028sep")
	fmt.Printf("[%# X]\n", "\u2028sep")
	fmt.Printf("[%10X]\n", "\u2028sep")
	fmt.Printf("[%-10X]\n", "\u2028sep")
	fmt.Printf("[%.3X]\n", "\u2028sep")
	fmt.Printf("[%.0X]\n", "\u2028sep")
	fmt.Printf("[%20.2X]\n", "\u2028sep")
	fmt.Printf("[% 12X]\n", "\u2028sep")
	fmt.Printf("[%#12X]\n", "\u2028sep")
	fmt.Printf("[%020X]\n", "\u2028sep")
	fmt.Printf("[%X]\n", "bell\a")
	fmt.Printf("[%+X]\n", "bell\a")
	fmt.Printf("[%-X]\n", "bell\a")
	fmt.Printf("[% X]\n", "bell\a")
	fmt.Printf("[%#X]\n", "bell\a")
	fmt.Printf("[%# X]\n", "bell\a")
	fmt.Printf("[%10X]\n", "bell\a")
	fmt.Printf("[%-10X]\n", "bell\a")
	fmt.Printf("[%.3X]\n", "bell\a")
	fmt.Printf("[%.0X]\n", "bell\a")
	fmt.Printf("[%20.2X]\n", "bell\a")
	fmt.Printf("[% 12X]\n", "bell\a")
	fmt.Printf("[%#12X]\n", "bell\a")
	fmt.Printf("[%020X]\n", "bell\a")
	fmt.Printf("[%X]\n", "\x7f")
	fmt.Printf("[%+X]\n", "\x7f")
	fmt.Printf("[%-X]\n", "\x7f")
	fmt.Printf("[% X]\n", "\x7f")
	fmt.Printf("[%#X]\n", "\x7f")
	fmt.Printf("[%# X]\n", "\x7f")
	fmt.Printf("[%10X]\n", "\x7f")
	fmt.Printf("[%-10X]\n", "\x7f")
	fmt.Printf("[%.3X]\n", "\x7f")
	fmt.Printf("[%.0X]\n", "\x7f")
	fmt.Printf("[%20.2X]\n", "\x7f")
	fmt.Printf("[% 12X]\n", "\x7f")
	fmt.Printf("[%#12X]\n", "\x7f")
	fmt.Printf("[%020X]\n", "\x7f")
	fmt.Printf("[%X]\n", "\x00nul")
	fmt.Printf("[%+X]\n", "\x00nul")
	fmt.Printf("[%-X]\n", "\x00nul")
	fmt.Printf("[% X]\n", "\x00nul")
	fmt.Printf("[%#X]\n", "\x00nul")
	fmt.Printf("[%# X]\n", "\x00nul")
	fmt.Printf("[%10X]\n", "\x00nul")
	fmt.Printf("[%-10X]\n", "\x00nul")
	fmt.Printf("[%.3X]\n", "\x00nul")
	fmt.Printf("[%.0X]\n", "\x00nul")
	fmt.Printf("[%20.2X]\n", "\x00nul")
	fmt.Printf("[% 12X]\n", "\x00nul")
	fmt.Printf("[%#12X]\n", "\x00nul")
	fmt.Printf("[%020X]\n", "\x00nul")
	fmt.Printf("[%X]\n", "\ufeffbom")
	fmt.Printf("[%+X]\n", "\ufeffbom")
	fmt.Printf("[%-X]\n", "\ufeffbom")
	fmt.Printf("[% X]\n", "\ufeffbom")
	fmt.Printf("[%#X]\n", "\ufeffbom")
	fmt.Printf("[%# X]\n", "\ufeffbom")
	fmt.Printf("[%10X]\n", "\ufeffbom")
	fmt.Printf("[%-10X]\n", "\ufeffbom")
	fmt.Printf("[%.3X]\n", "\ufeffbom")
	fmt.Printf("[%.0X]\n", "\ufeffbom")
	fmt.Printf("[%20.2X]\n", "\ufeffbom")
	fmt.Printf("[% 12X]\n", "\ufeffbom")
	fmt.Printf("[%#12X]\n", "\ufeffbom")
	fmt.Printf("[%020X]\n", "\ufeffbom")
	fmt.Printf("[%X]\n", "\x1b[0m")
	fmt.Printf("[%+X]\n", "\x1b[0m")
	fmt.Printf("[%-X]\n", "\x1b[0m")
	fmt.Printf("[% X]\n", "\x1b[0m")
	fmt.Printf("[%#X]\n", "\x1b[0m")
	fmt.Printf("[%# X]\n", "\x1b[0m")
	fmt.Printf("[%10X]\n", "\x1b[0m")
	fmt.Printf("[%-10X]\n", "\x1b[0m")
	fmt.Printf("[%.3X]\n", "\x1b[0m")
	fmt.Printf("[%.0X]\n", "\x1b[0m")
	fmt.Printf("[%20.2X]\n", "\x1b[0m")
	fmt.Printf("[% 12X]\n", "\x1b[0m")
	fmt.Printf("[%#12X]\n", "\x1b[0m")
	fmt.Printf("[%020X]\n", "\x1b[0m")
	fmt.Printf("[%X]\n", "\u0085next")
	fmt.Printf("[%+X]\n", "\u0085next")
	fmt.Printf("[%-X]\n", "\u0085next")
	fmt.Printf("[% X]\n", "\u0085next")
	fmt.Printf("[%#X]\n", "\u0085next")
	fmt.Printf("[%# X]\n", "\u0085next")
	fmt.Printf("[%10X]\n", "\u0085next")
	fmt.Printf("[%-10X]\n", "\u0085next")
	fmt.Printf("[%.3X]\n", "\u0085next")
	fmt.Printf("[%.0X]\n", "\u0085next")
	fmt.Printf("[%20.2X]\n", "\u0085next")
	fmt.Printf("[% 12X]\n", "\u0085next")
	fmt.Printf("[%#12X]\n", "\u0085next")
	fmt.Printf("[%020X]\n", "\u0085next")
}

// Output:
// [E280A8736570]
// [E280A8736570]
// [E280A8736570]
// [E2 80 A8 73 65 70]
// [0XE280A8736570]
// [0XE2 0X80 0XA8 0X73 0X65 0X70]
// [E280A8736570]
// [E280A8736570]
// [E280A8]
// []
// [                E280]
// [E2 80 A8 73 65 70]
// [0XE280A8736570]
// [00000000E280A8736570]
// [62656C6C07]
// [62656C6C07]
// [62656C6C07]
// [62 65 6C 6C 07]
// [0X62656C6C07]
// [0X62 0X65 0X6C 0X6C 0X07]
// [62656C6C07]
// [62656C6C07]
// [62656C]
// []
// [                6265]
// [62 65 6C 6C 07]
// [0X62656C6C07]
// [000000000062656C6C07]
// [7F]
// [7F]
// [7F]
// [7F]
// [0X7F]
// [0X7F]
// [        7F]
// [7F        ]
// [7F]
// []
// [                  7F]
// [          7F]
// [        0X7F]
// [0000000000000000007F]
// [006E756C]
// [006E756C]
// [006E756C]
// [00 6E 75 6C]
// [0X006E756C]
// [0X00 0X6E 0X75 0X6C]
// [  006E756C]
// [006E756C  ]
// [006E75]
// []
// [                006E]
// [ 00 6E 75 6C]
// [  0X006E756C]
// [000000000000006E756C]
// [EFBBBF626F6D]
// [EFBBBF626F6D]
// [EFBBBF626F6D]
// [EF BB BF 62 6F 6D]
// [0XEFBBBF626F6D]
// [0XEF 0XBB 0XBF 0X62 0X6F 0X6D]
// [EFBBBF626F6D]
// [EFBBBF626F6D]
// [EFBBBF]
// []
// [                EFBB]
// [EF BB BF 62 6F 6D]
// [0XEFBBBF626F6D]
// [00000000EFBBBF626F6D]
// [1B5B306D]
// [1B5B306D]
// [1B5B306D]
// [1B 5B 30 6D]
// [0X1B5B306D]
// [0X1B 0X5B 0X30 0X6D]
// [  1B5B306D]
// [1B5B306D  ]
// [1B5B30]
// []
// [                1B5B]
// [ 1B 5B 30 6D]
// [  0X1B5B306D]
// [0000000000001B5B306D]
// [C2856E657874]
// [C2856E657874]
// [C2856E657874]
// [C2 85 6E 65 78 74]
// [0XC2856E657874]
// [0XC2 0X85 0X6E 0X65 0X78 0X74]
// [C2856E657874]
// [C2856E657874]
// [C2856E]
// []
// [                C285]
// [C2 85 6E 65 78 74]
// [0XC2856E657874]
// [00000000C2856E657874]
