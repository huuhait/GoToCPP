// phase: 03

// string(r) of an integer is the UTF-8 of the rune r, and "�" for a value that
// is no code point: negative, a surrogate half, above U+10FFFF, or too big for a
// rune (no truncation), for integers of every size.
package main

import "fmt"

func bytes(s string) {
	fmt.Print(len(s), ":")
	for i := 0; i < len(s); i++ {
		fmt.Print(" ", s[i])
	}
	fmt.Println()
}

func main() {
	for _, r := range "Aé€\U0001F600\x00" {
		bytes(string(r))
	}
	var rs = "\u007f\u0080߿ࠀ퟿￿\U00010000\U0010ffff"
	for _, r := range rs {
		bytes(string(r))
	}
	var neg, lo, hi, big, min rune = -1, 0xD800, 0xDFFF, 0x110000, -2147483648
	bytes(string(neg))
	bytes(string(lo))
	bytes(string(hi))
	bytes(string(big))
	bytes(string(min))
	var b byte = 0xFF
	var i8 int8 = -1
	var u16 uint16 = 0xD800
	var u32 uint32 = 0xFFFFFFFF
	var i64 int64 = 0x100000041
	var u64 uint64 = 1<<64 - 1
	var u uint = 0x41
	var i int = 0x10FFFF
	var p uintptr = 0x263A
	bytes(string(b))
	bytes(string(i8))
	bytes(string(u16))
	bytes(string(u32))
	bytes(string(i64))
	bytes(string(u64))
	bytes(string(u))
	bytes(string(i))
	bytes(string(p))
	fmt.Println(string(rune(0x263A))+string(u)+string(rune(b-0xBF)), string(neg) == "�")
}
