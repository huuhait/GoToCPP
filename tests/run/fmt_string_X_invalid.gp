// phase: 05

// Go's %X of strings (invalid) with flags, width and precision (runes, not bytes).
package main

import "fmt"

func main() {
	fmt.Printf("[%X]\n", "\xff")
	fmt.Printf("[%+X]\n", "\xff")
	fmt.Printf("[%-X]\n", "\xff")
	fmt.Printf("[% X]\n", "\xff")
	fmt.Printf("[%#X]\n", "\xff")
	fmt.Printf("[%# X]\n", "\xff")
	fmt.Printf("[%10X]\n", "\xff")
	fmt.Printf("[%-10X]\n", "\xff")
	fmt.Printf("[%.3X]\n", "\xff")
	fmt.Printf("[%.0X]\n", "\xff")
	fmt.Printf("[%20.2X]\n", "\xff")
	fmt.Printf("[% 12X]\n", "\xff")
	fmt.Printf("[%#12X]\n", "\xff")
	fmt.Printf("[%020X]\n", "\xff")
	fmt.Printf("[%X]\n", "ab\xc3")
	fmt.Printf("[%+X]\n", "ab\xc3")
	fmt.Printf("[%-X]\n", "ab\xc3")
	fmt.Printf("[% X]\n", "ab\xc3")
	fmt.Printf("[%#X]\n", "ab\xc3")
	fmt.Printf("[%# X]\n", "ab\xc3")
	fmt.Printf("[%10X]\n", "ab\xc3")
	fmt.Printf("[%-10X]\n", "ab\xc3")
	fmt.Printf("[%.3X]\n", "ab\xc3")
	fmt.Printf("[%.0X]\n", "ab\xc3")
	fmt.Printf("[%20.2X]\n", "ab\xc3")
	fmt.Printf("[% 12X]\n", "ab\xc3")
	fmt.Printf("[%#12X]\n", "ab\xc3")
	fmt.Printf("[%020X]\n", "ab\xc3")
	fmt.Printf("[%X]\n", "\xed\xa0\x80")
	fmt.Printf("[%+X]\n", "\xed\xa0\x80")
	fmt.Printf("[%-X]\n", "\xed\xa0\x80")
	fmt.Printf("[% X]\n", "\xed\xa0\x80")
	fmt.Printf("[%#X]\n", "\xed\xa0\x80")
	fmt.Printf("[%# X]\n", "\xed\xa0\x80")
	fmt.Printf("[%10X]\n", "\xed\xa0\x80")
	fmt.Printf("[%-10X]\n", "\xed\xa0\x80")
	fmt.Printf("[%.3X]\n", "\xed\xa0\x80")
	fmt.Printf("[%.0X]\n", "\xed\xa0\x80")
	fmt.Printf("[%20.2X]\n", "\xed\xa0\x80")
	fmt.Printf("[% 12X]\n", "\xed\xa0\x80")
	fmt.Printf("[%#12X]\n", "\xed\xa0\x80")
	fmt.Printf("[%020X]\n", "\xed\xa0\x80")
	fmt.Printf("[%X]\n", "\xf4\x90\x80\x80")
	fmt.Printf("[%+X]\n", "\xf4\x90\x80\x80")
	fmt.Printf("[%-X]\n", "\xf4\x90\x80\x80")
	fmt.Printf("[% X]\n", "\xf4\x90\x80\x80")
	fmt.Printf("[%#X]\n", "\xf4\x90\x80\x80")
	fmt.Printf("[%# X]\n", "\xf4\x90\x80\x80")
	fmt.Printf("[%10X]\n", "\xf4\x90\x80\x80")
	fmt.Printf("[%-10X]\n", "\xf4\x90\x80\x80")
	fmt.Printf("[%.3X]\n", "\xf4\x90\x80\x80")
	fmt.Printf("[%.0X]\n", "\xf4\x90\x80\x80")
	fmt.Printf("[%20.2X]\n", "\xf4\x90\x80\x80")
	fmt.Printf("[% 12X]\n", "\xf4\x90\x80\x80")
	fmt.Printf("[%#12X]\n", "\xf4\x90\x80\x80")
	fmt.Printf("[%020X]\n", "\xf4\x90\x80\x80")
	fmt.Printf("[%X]\n", "a\xffb")
	fmt.Printf("[%+X]\n", "a\xffb")
	fmt.Printf("[%-X]\n", "a\xffb")
	fmt.Printf("[% X]\n", "a\xffb")
	fmt.Printf("[%#X]\n", "a\xffb")
	fmt.Printf("[%# X]\n", "a\xffb")
	fmt.Printf("[%10X]\n", "a\xffb")
	fmt.Printf("[%-10X]\n", "a\xffb")
	fmt.Printf("[%.3X]\n", "a\xffb")
	fmt.Printf("[%.0X]\n", "a\xffb")
	fmt.Printf("[%20.2X]\n", "a\xffb")
	fmt.Printf("[% 12X]\n", "a\xffb")
	fmt.Printf("[%#12X]\n", "a\xffb")
	fmt.Printf("[%020X]\n", "a\xffb")
}

// Output:
// [FF]
// [FF]
// [FF]
// [FF]
// [0XFF]
// [0XFF]
// [        FF]
// [FF        ]
// [FF]
// []
// [                  FF]
// [          FF]
// [        0XFF]
// [000000000000000000FF]
// [6162C3]
// [6162C3]
// [6162C3]
// [61 62 C3]
// [0X6162C3]
// [0X61 0X62 0XC3]
// [    6162C3]
// [6162C3    ]
// [6162C3]
// []
// [                6162]
// [    61 62 C3]
// [    0X6162C3]
// [000000000000006162C3]
// [EDA080]
// [EDA080]
// [EDA080]
// [ED A0 80]
// [0XEDA080]
// [0XED 0XA0 0X80]
// [    EDA080]
// [EDA080    ]
// [EDA080]
// []
// [                EDA0]
// [    ED A0 80]
// [    0XEDA080]
// [00000000000000EDA080]
// [F4908080]
// [F4908080]
// [F4908080]
// [F4 90 80 80]
// [0XF4908080]
// [0XF4 0X90 0X80 0X80]
// [  F4908080]
// [F4908080  ]
// [F49080]
// []
// [                F490]
// [ F4 90 80 80]
// [  0XF4908080]
// [000000000000F4908080]
// [61FF62]
// [61FF62]
// [61FF62]
// [61 FF 62]
// [0X61FF62]
// [0X61 0XFF 0X62]
// [    61FF62]
// [61FF62    ]
// [61FF62]
// []
// [                61FF]
// [    61 FF 62]
// [    0X61FF62]
// [0000000000000061FF62]
