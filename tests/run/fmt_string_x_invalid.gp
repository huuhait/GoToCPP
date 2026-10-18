// phase: 05

// Go's %x of strings (invalid) with flags, width and precision (runes, not bytes).
package main

import "fmt"

func main() {
	fmt.Printf("[%x]\n", "\xff")
	fmt.Printf("[%+x]\n", "\xff")
	fmt.Printf("[%-x]\n", "\xff")
	fmt.Printf("[% x]\n", "\xff")
	fmt.Printf("[%#x]\n", "\xff")
	fmt.Printf("[%# x]\n", "\xff")
	fmt.Printf("[%10x]\n", "\xff")
	fmt.Printf("[%-10x]\n", "\xff")
	fmt.Printf("[%.3x]\n", "\xff")
	fmt.Printf("[%.0x]\n", "\xff")
	fmt.Printf("[%20.2x]\n", "\xff")
	fmt.Printf("[% 12x]\n", "\xff")
	fmt.Printf("[%#12x]\n", "\xff")
	fmt.Printf("[%020x]\n", "\xff")
	fmt.Printf("[%x]\n", "ab\xc3")
	fmt.Printf("[%+x]\n", "ab\xc3")
	fmt.Printf("[%-x]\n", "ab\xc3")
	fmt.Printf("[% x]\n", "ab\xc3")
	fmt.Printf("[%#x]\n", "ab\xc3")
	fmt.Printf("[%# x]\n", "ab\xc3")
	fmt.Printf("[%10x]\n", "ab\xc3")
	fmt.Printf("[%-10x]\n", "ab\xc3")
	fmt.Printf("[%.3x]\n", "ab\xc3")
	fmt.Printf("[%.0x]\n", "ab\xc3")
	fmt.Printf("[%20.2x]\n", "ab\xc3")
	fmt.Printf("[% 12x]\n", "ab\xc3")
	fmt.Printf("[%#12x]\n", "ab\xc3")
	fmt.Printf("[%020x]\n", "ab\xc3")
	fmt.Printf("[%x]\n", "\xed\xa0\x80")
	fmt.Printf("[%+x]\n", "\xed\xa0\x80")
	fmt.Printf("[%-x]\n", "\xed\xa0\x80")
	fmt.Printf("[% x]\n", "\xed\xa0\x80")
	fmt.Printf("[%#x]\n", "\xed\xa0\x80")
	fmt.Printf("[%# x]\n", "\xed\xa0\x80")
	fmt.Printf("[%10x]\n", "\xed\xa0\x80")
	fmt.Printf("[%-10x]\n", "\xed\xa0\x80")
	fmt.Printf("[%.3x]\n", "\xed\xa0\x80")
	fmt.Printf("[%.0x]\n", "\xed\xa0\x80")
	fmt.Printf("[%20.2x]\n", "\xed\xa0\x80")
	fmt.Printf("[% 12x]\n", "\xed\xa0\x80")
	fmt.Printf("[%#12x]\n", "\xed\xa0\x80")
	fmt.Printf("[%020x]\n", "\xed\xa0\x80")
	fmt.Printf("[%x]\n", "\xf4\x90\x80\x80")
	fmt.Printf("[%+x]\n", "\xf4\x90\x80\x80")
	fmt.Printf("[%-x]\n", "\xf4\x90\x80\x80")
	fmt.Printf("[% x]\n", "\xf4\x90\x80\x80")
	fmt.Printf("[%#x]\n", "\xf4\x90\x80\x80")
	fmt.Printf("[%# x]\n", "\xf4\x90\x80\x80")
	fmt.Printf("[%10x]\n", "\xf4\x90\x80\x80")
	fmt.Printf("[%-10x]\n", "\xf4\x90\x80\x80")
	fmt.Printf("[%.3x]\n", "\xf4\x90\x80\x80")
	fmt.Printf("[%.0x]\n", "\xf4\x90\x80\x80")
	fmt.Printf("[%20.2x]\n", "\xf4\x90\x80\x80")
	fmt.Printf("[% 12x]\n", "\xf4\x90\x80\x80")
	fmt.Printf("[%#12x]\n", "\xf4\x90\x80\x80")
	fmt.Printf("[%020x]\n", "\xf4\x90\x80\x80")
	fmt.Printf("[%x]\n", "a\xffb")
	fmt.Printf("[%+x]\n", "a\xffb")
	fmt.Printf("[%-x]\n", "a\xffb")
	fmt.Printf("[% x]\n", "a\xffb")
	fmt.Printf("[%#x]\n", "a\xffb")
	fmt.Printf("[%# x]\n", "a\xffb")
	fmt.Printf("[%10x]\n", "a\xffb")
	fmt.Printf("[%-10x]\n", "a\xffb")
	fmt.Printf("[%.3x]\n", "a\xffb")
	fmt.Printf("[%.0x]\n", "a\xffb")
	fmt.Printf("[%20.2x]\n", "a\xffb")
	fmt.Printf("[% 12x]\n", "a\xffb")
	fmt.Printf("[%#12x]\n", "a\xffb")
	fmt.Printf("[%020x]\n", "a\xffb")
}

// Output:
// [ff]
// [ff]
// [ff]
// [ff]
// [0xff]
// [0xff]
// [        ff]
// [ff        ]
// [ff]
// []
// [                  ff]
// [          ff]
// [        0xff]
// [000000000000000000ff]
// [6162c3]
// [6162c3]
// [6162c3]
// [61 62 c3]
// [0x6162c3]
// [0x61 0x62 0xc3]
// [    6162c3]
// [6162c3    ]
// [6162c3]
// []
// [                6162]
// [    61 62 c3]
// [    0x6162c3]
// [000000000000006162c3]
// [eda080]
// [eda080]
// [eda080]
// [ed a0 80]
// [0xeda080]
// [0xed 0xa0 0x80]
// [    eda080]
// [eda080    ]
// [eda080]
// []
// [                eda0]
// [    ed a0 80]
// [    0xeda080]
// [00000000000000eda080]
// [f4908080]
// [f4908080]
// [f4908080]
// [f4 90 80 80]
// [0xf4908080]
// [0xf4 0x90 0x80 0x80]
// [  f4908080]
// [f4908080  ]
// [f49080]
// []
// [                f490]
// [ f4 90 80 80]
// [  0xf4908080]
// [000000000000f4908080]
// [61ff62]
// [61ff62]
// [61ff62]
// [61 ff 62]
// [0x61ff62]
// [0x61 0xff 0x62]
// [    61ff62]
// [61ff62    ]
// [61ff62]
// []
// [                61ff]
// [    61 ff 62]
// [    0x61ff62]
// [0000000000000061ff62]
