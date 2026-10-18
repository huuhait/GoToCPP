// phase: 05

// Go's %x of strings (controls) with flags, width and precision (runes, not bytes).
package main

import "fmt"

func main() {
	fmt.Printf("[%x]\n", "\u2028sep")
	fmt.Printf("[%+x]\n", "\u2028sep")
	fmt.Printf("[%-x]\n", "\u2028sep")
	fmt.Printf("[% x]\n", "\u2028sep")
	fmt.Printf("[%#x]\n", "\u2028sep")
	fmt.Printf("[%# x]\n", "\u2028sep")
	fmt.Printf("[%10x]\n", "\u2028sep")
	fmt.Printf("[%-10x]\n", "\u2028sep")
	fmt.Printf("[%.3x]\n", "\u2028sep")
	fmt.Printf("[%.0x]\n", "\u2028sep")
	fmt.Printf("[%20.2x]\n", "\u2028sep")
	fmt.Printf("[% 12x]\n", "\u2028sep")
	fmt.Printf("[%#12x]\n", "\u2028sep")
	fmt.Printf("[%020x]\n", "\u2028sep")
	fmt.Printf("[%x]\n", "bell\a")
	fmt.Printf("[%+x]\n", "bell\a")
	fmt.Printf("[%-x]\n", "bell\a")
	fmt.Printf("[% x]\n", "bell\a")
	fmt.Printf("[%#x]\n", "bell\a")
	fmt.Printf("[%# x]\n", "bell\a")
	fmt.Printf("[%10x]\n", "bell\a")
	fmt.Printf("[%-10x]\n", "bell\a")
	fmt.Printf("[%.3x]\n", "bell\a")
	fmt.Printf("[%.0x]\n", "bell\a")
	fmt.Printf("[%20.2x]\n", "bell\a")
	fmt.Printf("[% 12x]\n", "bell\a")
	fmt.Printf("[%#12x]\n", "bell\a")
	fmt.Printf("[%020x]\n", "bell\a")
	fmt.Printf("[%x]\n", "\x7f")
	fmt.Printf("[%+x]\n", "\x7f")
	fmt.Printf("[%-x]\n", "\x7f")
	fmt.Printf("[% x]\n", "\x7f")
	fmt.Printf("[%#x]\n", "\x7f")
	fmt.Printf("[%# x]\n", "\x7f")
	fmt.Printf("[%10x]\n", "\x7f")
	fmt.Printf("[%-10x]\n", "\x7f")
	fmt.Printf("[%.3x]\n", "\x7f")
	fmt.Printf("[%.0x]\n", "\x7f")
	fmt.Printf("[%20.2x]\n", "\x7f")
	fmt.Printf("[% 12x]\n", "\x7f")
	fmt.Printf("[%#12x]\n", "\x7f")
	fmt.Printf("[%020x]\n", "\x7f")
	fmt.Printf("[%x]\n", "\x00nul")
	fmt.Printf("[%+x]\n", "\x00nul")
	fmt.Printf("[%-x]\n", "\x00nul")
	fmt.Printf("[% x]\n", "\x00nul")
	fmt.Printf("[%#x]\n", "\x00nul")
	fmt.Printf("[%# x]\n", "\x00nul")
	fmt.Printf("[%10x]\n", "\x00nul")
	fmt.Printf("[%-10x]\n", "\x00nul")
	fmt.Printf("[%.3x]\n", "\x00nul")
	fmt.Printf("[%.0x]\n", "\x00nul")
	fmt.Printf("[%20.2x]\n", "\x00nul")
	fmt.Printf("[% 12x]\n", "\x00nul")
	fmt.Printf("[%#12x]\n", "\x00nul")
	fmt.Printf("[%020x]\n", "\x00nul")
	fmt.Printf("[%x]\n", "\ufeffbom")
	fmt.Printf("[%+x]\n", "\ufeffbom")
	fmt.Printf("[%-x]\n", "\ufeffbom")
	fmt.Printf("[% x]\n", "\ufeffbom")
	fmt.Printf("[%#x]\n", "\ufeffbom")
	fmt.Printf("[%# x]\n", "\ufeffbom")
	fmt.Printf("[%10x]\n", "\ufeffbom")
	fmt.Printf("[%-10x]\n", "\ufeffbom")
	fmt.Printf("[%.3x]\n", "\ufeffbom")
	fmt.Printf("[%.0x]\n", "\ufeffbom")
	fmt.Printf("[%20.2x]\n", "\ufeffbom")
	fmt.Printf("[% 12x]\n", "\ufeffbom")
	fmt.Printf("[%#12x]\n", "\ufeffbom")
	fmt.Printf("[%020x]\n", "\ufeffbom")
	fmt.Printf("[%x]\n", "\x1b[0m")
	fmt.Printf("[%+x]\n", "\x1b[0m")
	fmt.Printf("[%-x]\n", "\x1b[0m")
	fmt.Printf("[% x]\n", "\x1b[0m")
	fmt.Printf("[%#x]\n", "\x1b[0m")
	fmt.Printf("[%# x]\n", "\x1b[0m")
	fmt.Printf("[%10x]\n", "\x1b[0m")
	fmt.Printf("[%-10x]\n", "\x1b[0m")
	fmt.Printf("[%.3x]\n", "\x1b[0m")
	fmt.Printf("[%.0x]\n", "\x1b[0m")
	fmt.Printf("[%20.2x]\n", "\x1b[0m")
	fmt.Printf("[% 12x]\n", "\x1b[0m")
	fmt.Printf("[%#12x]\n", "\x1b[0m")
	fmt.Printf("[%020x]\n", "\x1b[0m")
	fmt.Printf("[%x]\n", "\u0085next")
	fmt.Printf("[%+x]\n", "\u0085next")
	fmt.Printf("[%-x]\n", "\u0085next")
	fmt.Printf("[% x]\n", "\u0085next")
	fmt.Printf("[%#x]\n", "\u0085next")
	fmt.Printf("[%# x]\n", "\u0085next")
	fmt.Printf("[%10x]\n", "\u0085next")
	fmt.Printf("[%-10x]\n", "\u0085next")
	fmt.Printf("[%.3x]\n", "\u0085next")
	fmt.Printf("[%.0x]\n", "\u0085next")
	fmt.Printf("[%20.2x]\n", "\u0085next")
	fmt.Printf("[% 12x]\n", "\u0085next")
	fmt.Printf("[%#12x]\n", "\u0085next")
	fmt.Printf("[%020x]\n", "\u0085next")
}

// Output:
// [e280a8736570]
// [e280a8736570]
// [e280a8736570]
// [e2 80 a8 73 65 70]
// [0xe280a8736570]
// [0xe2 0x80 0xa8 0x73 0x65 0x70]
// [e280a8736570]
// [e280a8736570]
// [e280a8]
// []
// [                e280]
// [e2 80 a8 73 65 70]
// [0xe280a8736570]
// [00000000e280a8736570]
// [62656c6c07]
// [62656c6c07]
// [62656c6c07]
// [62 65 6c 6c 07]
// [0x62656c6c07]
// [0x62 0x65 0x6c 0x6c 0x07]
// [62656c6c07]
// [62656c6c07]
// [62656c]
// []
// [                6265]
// [62 65 6c 6c 07]
// [0x62656c6c07]
// [000000000062656c6c07]
// [7f]
// [7f]
// [7f]
// [7f]
// [0x7f]
// [0x7f]
// [        7f]
// [7f        ]
// [7f]
// []
// [                  7f]
// [          7f]
// [        0x7f]
// [0000000000000000007f]
// [006e756c]
// [006e756c]
// [006e756c]
// [00 6e 75 6c]
// [0x006e756c]
// [0x00 0x6e 0x75 0x6c]
// [  006e756c]
// [006e756c  ]
// [006e75]
// []
// [                006e]
// [ 00 6e 75 6c]
// [  0x006e756c]
// [000000000000006e756c]
// [efbbbf626f6d]
// [efbbbf626f6d]
// [efbbbf626f6d]
// [ef bb bf 62 6f 6d]
// [0xefbbbf626f6d]
// [0xef 0xbb 0xbf 0x62 0x6f 0x6d]
// [efbbbf626f6d]
// [efbbbf626f6d]
// [efbbbf]
// []
// [                efbb]
// [ef bb bf 62 6f 6d]
// [0xefbbbf626f6d]
// [00000000efbbbf626f6d]
// [1b5b306d]
// [1b5b306d]
// [1b5b306d]
// [1b 5b 30 6d]
// [0x1b5b306d]
// [0x1b 0x5b 0x30 0x6d]
// [  1b5b306d]
// [1b5b306d  ]
// [1b5b30]
// []
// [                1b5b]
// [ 1b 5b 30 6d]
// [  0x1b5b306d]
// [0000000000001b5b306d]
// [c2856e657874]
// [c2856e657874]
// [c2856e657874]
// [c2 85 6e 65 78 74]
// [0xc2856e657874]
// [0xc2 0x85 0x6e 0x65 0x78 0x74]
// [c2856e657874]
// [c2856e657874]
// [c2856e]
// []
// [                c285]
// [c2 85 6e 65 78 74]
// [0xc2856e657874]
// [00000000c2856e657874]
