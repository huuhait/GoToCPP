// phase: 09

// unicode/utf16 (ported): surrogate pairs, encoding and decoding of runes and strings, and the
// replacement character for lone surrogates.
package main

import (
	"fmt"
	"unicode/utf16"
)

func main() {
	for _, r := range []rune{'a', 0xe9, 0xd7ff, 0xe000, 0xffff, 0x10000, 0x1f600, 0x10ffff, 0xd800, 0x110000, -1} {
		r1, r2 := utf16.EncodeRune(r)
		fmt.Printf("%U -> %04x %04x surrogate %t decode %U\n", r, r1, r2, utf16.IsSurrogate(r), utf16.DecodeRune(r1, r2))
	}
	s := []rune("héllo \U0001F600 世界")
	enc := utf16.Encode(s)
	fmt.Printf("%04x\n", enc)
	fmt.Println(string(utf16.Decode(enc)), len(s), len(enc))
	fmt.Printf("%04x\n", utf16.AppendRune([]uint16{1}, 0x1f600))
	fmt.Printf("%q\n", string(utf16.Decode([]uint16{0x61, 0xd800, 0x62, 0xdc00, 0xd83d, 0xde00, 0xd83d})))
	fmt.Println(utf16.RuneLen(0x41), utf16.RuneLen(0x1f600), utf16.RuneLen(0xd800), utf16.RuneLen(-1))
}

// Output:
// U+0061 -> fffd fffd surrogate false decode U+FFFD
// U+00E9 -> fffd fffd surrogate false decode U+FFFD
// U+D7FF -> fffd fffd surrogate false decode U+FFFD
// U+E000 -> fffd fffd surrogate false decode U+FFFD
// U+FFFF -> fffd fffd surrogate false decode U+FFFD
// U+10000 -> d800 dc00 surrogate false decode U+10000
// U+1F600 -> d83d de00 surrogate false decode U+1F600
// U+10FFFF -> dbff dfff surrogate false decode U+10FFFF
// U+D800 -> fffd fffd surrogate true decode U+FFFD
// U+110000 -> fffd fffd surrogate false decode U+FFFD
// U+FFFFFFFFFFFFFFFF -> fffd fffd surrogate false decode U+FFFD
// [0068 00e9 006c 006c 006f 0020 d83d de00 0020 4e16 754c]
// héllo 😀 世界 10 11
// [0001 d83d de00]
// "a�b�😀�"
// 1 2 -1 -1
