// phase: 05

// Go's %X of strings (unicode) with flags, width and precision (runes, not bytes).
package main

import "fmt"

func main() {
	fmt.Printf("[%X]\n", "héllo")
	fmt.Printf("[%+X]\n", "héllo")
	fmt.Printf("[%-X]\n", "héllo")
	fmt.Printf("[% X]\n", "héllo")
	fmt.Printf("[%#X]\n", "héllo")
	fmt.Printf("[%# X]\n", "héllo")
	fmt.Printf("[%10X]\n", "héllo")
	fmt.Printf("[%-10X]\n", "héllo")
	fmt.Printf("[%.3X]\n", "héllo")
	fmt.Printf("[%.0X]\n", "héllo")
	fmt.Printf("[%20.2X]\n", "héllo")
	fmt.Printf("[% 12X]\n", "héllo")
	fmt.Printf("[%#12X]\n", "héllo")
	fmt.Printf("[%020X]\n", "héllo")
	fmt.Printf("[%X]\n", "日本語")
	fmt.Printf("[%+X]\n", "日本語")
	fmt.Printf("[%-X]\n", "日本語")
	fmt.Printf("[% X]\n", "日本語")
	fmt.Printf("[%#X]\n", "日本語")
	fmt.Printf("[%# X]\n", "日本語")
	fmt.Printf("[%10X]\n", "日本語")
	fmt.Printf("[%-10X]\n", "日本語")
	fmt.Printf("[%.3X]\n", "日本語")
	fmt.Printf("[%.0X]\n", "日本語")
	fmt.Printf("[%20.2X]\n", "日本語")
	fmt.Printf("[% 12X]\n", "日本語")
	fmt.Printf("[%#12X]\n", "日本語")
	fmt.Printf("[%020X]\n", "日本語")
	fmt.Printf("[%X]\n", "😀 smile")
	fmt.Printf("[%+X]\n", "😀 smile")
	fmt.Printf("[%-X]\n", "😀 smile")
	fmt.Printf("[% X]\n", "😀 smile")
	fmt.Printf("[%#X]\n", "😀 smile")
	fmt.Printf("[%# X]\n", "😀 smile")
	fmt.Printf("[%10X]\n", "😀 smile")
	fmt.Printf("[%-10X]\n", "😀 smile")
	fmt.Printf("[%.3X]\n", "😀 smile")
	fmt.Printf("[%.0X]\n", "😀 smile")
	fmt.Printf("[%20.2X]\n", "😀 smile")
	fmt.Printf("[% 12X]\n", "😀 smile")
	fmt.Printf("[%#12X]\n", "😀 smile")
	fmt.Printf("[%020X]\n", "😀 smile")
	fmt.Printf("[%X]\n", "\u00a0nbsp")
	fmt.Printf("[%+X]\n", "\u00a0nbsp")
	fmt.Printf("[%-X]\n", "\u00a0nbsp")
	fmt.Printf("[% X]\n", "\u00a0nbsp")
	fmt.Printf("[%#X]\n", "\u00a0nbsp")
	fmt.Printf("[%# X]\n", "\u00a0nbsp")
	fmt.Printf("[%10X]\n", "\u00a0nbsp")
	fmt.Printf("[%-10X]\n", "\u00a0nbsp")
	fmt.Printf("[%.3X]\n", "\u00a0nbsp")
	fmt.Printf("[%.0X]\n", "\u00a0nbsp")
	fmt.Printf("[%20.2X]\n", "\u00a0nbsp")
	fmt.Printf("[% 12X]\n", "\u00a0nbsp")
	fmt.Printf("[%#12X]\n", "\u00a0nbsp")
	fmt.Printf("[%020X]\n", "\u00a0nbsp")
	fmt.Printf("[%X]\n", "zero\u200bwidth")
	fmt.Printf("[%+X]\n", "zero\u200bwidth")
	fmt.Printf("[%-X]\n", "zero\u200bwidth")
	fmt.Printf("[% X]\n", "zero\u200bwidth")
	fmt.Printf("[%#X]\n", "zero\u200bwidth")
	fmt.Printf("[%# X]\n", "zero\u200bwidth")
	fmt.Printf("[%10X]\n", "zero\u200bwidth")
	fmt.Printf("[%-10X]\n", "zero\u200bwidth")
	fmt.Printf("[%.3X]\n", "zero\u200bwidth")
	fmt.Printf("[%.0X]\n", "zero\u200bwidth")
	fmt.Printf("[%20.2X]\n", "zero\u200bwidth")
	fmt.Printf("[% 12X]\n", "zero\u200bwidth")
	fmt.Printf("[%#12X]\n", "zero\u200bwidth")
	fmt.Printf("[%020X]\n", "zero\u200bwidth")
}

// Output:
// [68C3A96C6C6F]
// [68C3A96C6C6F]
// [68C3A96C6C6F]
// [68 C3 A9 6C 6C 6F]
// [0X68C3A96C6C6F]
// [0X68 0XC3 0XA9 0X6C 0X6C 0X6F]
// [68C3A96C6C6F]
// [68C3A96C6C6F]
// [68C3A9]
// []
// [                68C3]
// [68 C3 A9 6C 6C 6F]
// [0X68C3A96C6C6F]
// [0000000068C3A96C6C6F]
// [E697A5E69CACE8AA9E]
// [E697A5E69CACE8AA9E]
// [E697A5E69CACE8AA9E]
// [E6 97 A5 E6 9C AC E8 AA 9E]
// [0XE697A5E69CACE8AA9E]
// [0XE6 0X97 0XA5 0XE6 0X9C 0XAC 0XE8 0XAA 0X9E]
// [E697A5E69CACE8AA9E]
// [E697A5E69CACE8AA9E]
// [E697A5]
// []
// [                E697]
// [E6 97 A5 E6 9C AC E8 AA 9E]
// [0XE697A5E69CACE8AA9E]
// [00E697A5E69CACE8AA9E]
// [F09F988020736D696C65]
// [F09F988020736D696C65]
// [F09F988020736D696C65]
// [F0 9F 98 80 20 73 6D 69 6C 65]
// [0XF09F988020736D696C65]
// [0XF0 0X9F 0X98 0X80 0X20 0X73 0X6D 0X69 0X6C 0X65]
// [F09F988020736D696C65]
// [F09F988020736D696C65]
// [F09F98]
// []
// [                F09F]
// [F0 9F 98 80 20 73 6D 69 6C 65]
// [0XF09F988020736D696C65]
// [F09F988020736D696C65]
// [C2A06E627370]
// [C2A06E627370]
// [C2A06E627370]
// [C2 A0 6E 62 73 70]
// [0XC2A06E627370]
// [0XC2 0XA0 0X6E 0X62 0X73 0X70]
// [C2A06E627370]
// [C2A06E627370]
// [C2A06E]
// []
// [                C2A0]
// [C2 A0 6E 62 73 70]
// [0XC2A06E627370]
// [00000000C2A06E627370]
// [7A65726FE2808B7769647468]
// [7A65726FE2808B7769647468]
// [7A65726FE2808B7769647468]
// [7A 65 72 6F E2 80 8B 77 69 64 74 68]
// [0X7A65726FE2808B7769647468]
// [0X7A 0X65 0X72 0X6F 0XE2 0X80 0X8B 0X77 0X69 0X64 0X74 0X68]
// [7A65726FE2808B7769647468]
// [7A65726FE2808B7769647468]
// [7A6572]
// []
// [                7A65]
// [7A 65 72 6F E2 80 8B 77 69 64 74 68]
// [0X7A65726FE2808B7769647468]
// [7A65726FE2808B7769647468]
