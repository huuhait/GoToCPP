// phase: 05

// Go's %x of strings (unicode) with flags, width and precision (runes, not bytes).
package main

import "fmt"

func main() {
	fmt.Printf("[%x]\n", "héllo")
	fmt.Printf("[%+x]\n", "héllo")
	fmt.Printf("[%-x]\n", "héllo")
	fmt.Printf("[% x]\n", "héllo")
	fmt.Printf("[%#x]\n", "héllo")
	fmt.Printf("[%# x]\n", "héllo")
	fmt.Printf("[%10x]\n", "héllo")
	fmt.Printf("[%-10x]\n", "héllo")
	fmt.Printf("[%.3x]\n", "héllo")
	fmt.Printf("[%.0x]\n", "héllo")
	fmt.Printf("[%20.2x]\n", "héllo")
	fmt.Printf("[% 12x]\n", "héllo")
	fmt.Printf("[%#12x]\n", "héllo")
	fmt.Printf("[%020x]\n", "héllo")
	fmt.Printf("[%x]\n", "日本語")
	fmt.Printf("[%+x]\n", "日本語")
	fmt.Printf("[%-x]\n", "日本語")
	fmt.Printf("[% x]\n", "日本語")
	fmt.Printf("[%#x]\n", "日本語")
	fmt.Printf("[%# x]\n", "日本語")
	fmt.Printf("[%10x]\n", "日本語")
	fmt.Printf("[%-10x]\n", "日本語")
	fmt.Printf("[%.3x]\n", "日本語")
	fmt.Printf("[%.0x]\n", "日本語")
	fmt.Printf("[%20.2x]\n", "日本語")
	fmt.Printf("[% 12x]\n", "日本語")
	fmt.Printf("[%#12x]\n", "日本語")
	fmt.Printf("[%020x]\n", "日本語")
	fmt.Printf("[%x]\n", "😀 smile")
	fmt.Printf("[%+x]\n", "😀 smile")
	fmt.Printf("[%-x]\n", "😀 smile")
	fmt.Printf("[% x]\n", "😀 smile")
	fmt.Printf("[%#x]\n", "😀 smile")
	fmt.Printf("[%# x]\n", "😀 smile")
	fmt.Printf("[%10x]\n", "😀 smile")
	fmt.Printf("[%-10x]\n", "😀 smile")
	fmt.Printf("[%.3x]\n", "😀 smile")
	fmt.Printf("[%.0x]\n", "😀 smile")
	fmt.Printf("[%20.2x]\n", "😀 smile")
	fmt.Printf("[% 12x]\n", "😀 smile")
	fmt.Printf("[%#12x]\n", "😀 smile")
	fmt.Printf("[%020x]\n", "😀 smile")
	fmt.Printf("[%x]\n", "\u00a0nbsp")
	fmt.Printf("[%+x]\n", "\u00a0nbsp")
	fmt.Printf("[%-x]\n", "\u00a0nbsp")
	fmt.Printf("[% x]\n", "\u00a0nbsp")
	fmt.Printf("[%#x]\n", "\u00a0nbsp")
	fmt.Printf("[%# x]\n", "\u00a0nbsp")
	fmt.Printf("[%10x]\n", "\u00a0nbsp")
	fmt.Printf("[%-10x]\n", "\u00a0nbsp")
	fmt.Printf("[%.3x]\n", "\u00a0nbsp")
	fmt.Printf("[%.0x]\n", "\u00a0nbsp")
	fmt.Printf("[%20.2x]\n", "\u00a0nbsp")
	fmt.Printf("[% 12x]\n", "\u00a0nbsp")
	fmt.Printf("[%#12x]\n", "\u00a0nbsp")
	fmt.Printf("[%020x]\n", "\u00a0nbsp")
	fmt.Printf("[%x]\n", "zero\u200bwidth")
	fmt.Printf("[%+x]\n", "zero\u200bwidth")
	fmt.Printf("[%-x]\n", "zero\u200bwidth")
	fmt.Printf("[% x]\n", "zero\u200bwidth")
	fmt.Printf("[%#x]\n", "zero\u200bwidth")
	fmt.Printf("[%# x]\n", "zero\u200bwidth")
	fmt.Printf("[%10x]\n", "zero\u200bwidth")
	fmt.Printf("[%-10x]\n", "zero\u200bwidth")
	fmt.Printf("[%.3x]\n", "zero\u200bwidth")
	fmt.Printf("[%.0x]\n", "zero\u200bwidth")
	fmt.Printf("[%20.2x]\n", "zero\u200bwidth")
	fmt.Printf("[% 12x]\n", "zero\u200bwidth")
	fmt.Printf("[%#12x]\n", "zero\u200bwidth")
	fmt.Printf("[%020x]\n", "zero\u200bwidth")
}

// Output:
// [68c3a96c6c6f]
// [68c3a96c6c6f]
// [68c3a96c6c6f]
// [68 c3 a9 6c 6c 6f]
// [0x68c3a96c6c6f]
// [0x68 0xc3 0xa9 0x6c 0x6c 0x6f]
// [68c3a96c6c6f]
// [68c3a96c6c6f]
// [68c3a9]
// []
// [                68c3]
// [68 c3 a9 6c 6c 6f]
// [0x68c3a96c6c6f]
// [0000000068c3a96c6c6f]
// [e697a5e69cace8aa9e]
// [e697a5e69cace8aa9e]
// [e697a5e69cace8aa9e]
// [e6 97 a5 e6 9c ac e8 aa 9e]
// [0xe697a5e69cace8aa9e]
// [0xe6 0x97 0xa5 0xe6 0x9c 0xac 0xe8 0xaa 0x9e]
// [e697a5e69cace8aa9e]
// [e697a5e69cace8aa9e]
// [e697a5]
// []
// [                e697]
// [e6 97 a5 e6 9c ac e8 aa 9e]
// [0xe697a5e69cace8aa9e]
// [00e697a5e69cace8aa9e]
// [f09f988020736d696c65]
// [f09f988020736d696c65]
// [f09f988020736d696c65]
// [f0 9f 98 80 20 73 6d 69 6c 65]
// [0xf09f988020736d696c65]
// [0xf0 0x9f 0x98 0x80 0x20 0x73 0x6d 0x69 0x6c 0x65]
// [f09f988020736d696c65]
// [f09f988020736d696c65]
// [f09f98]
// []
// [                f09f]
// [f0 9f 98 80 20 73 6d 69 6c 65]
// [0xf09f988020736d696c65]
// [f09f988020736d696c65]
// [c2a06e627370]
// [c2a06e627370]
// [c2a06e627370]
// [c2 a0 6e 62 73 70]
// [0xc2a06e627370]
// [0xc2 0xa0 0x6e 0x62 0x73 0x70]
// [c2a06e627370]
// [c2a06e627370]
// [c2a06e]
// []
// [                c2a0]
// [c2 a0 6e 62 73 70]
// [0xc2a06e627370]
// [00000000c2a06e627370]
// [7a65726fe2808b7769647468]
// [7a65726fe2808b7769647468]
// [7a65726fe2808b7769647468]
// [7a 65 72 6f e2 80 8b 77 69 64 74 68]
// [0x7a65726fe2808b7769647468]
// [0x7a 0x65 0x72 0x6f 0xe2 0x80 0x8b 0x77 0x69 0x64 0x74 0x68]
// [7a65726fe2808b7769647468]
// [7a65726fe2808b7769647468]
// [7a6572]
// []
// [                7a65]
// [7a 65 72 6f e2 80 8b 77 69 64 74 68]
// [0x7a65726fe2808b7769647468]
// [7a65726fe2808b7769647468]
