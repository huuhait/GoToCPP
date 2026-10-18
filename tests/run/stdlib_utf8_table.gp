// phase: 09

// unicode/utf8, ported from Go by tools/portstd: every function on a table of valid and broken
// strings (overlong forms, surrogates, truncated sequences, bytes that are never UTF-8), compared
// with real Go by TestRunVsGo.
package main

import (
	"fmt"
	"unicode/utf8"
)

var inputs = []string{
	"",
	"a",
	"hello, world",
	"héllo",
	"世界",
	"\U0001F600 and \U0010FFFF",
	"\xff",
	"abc\xffdef",
	"\xc0\x80",         // overlong NUL
	"\xe0\x80\x80",     // overlong
	"\xed\xa0\x80",     // a surrogate half, U+D800
	"\xf4\x90\x80\x80", // above U+10FFFF
	"\xe4\xb8",         // a truncated 3-byte sequence
	"\xf0\x9f\x98",     // a truncated 4-byte sequence
	"\x80\x80",         // continuation bytes alone
	"a\xe4\xb8",
	"�",
	"\x00\x7f\u0080߿ࠀ￿",
}

func main() {
	for i, s := range inputs {
		b := []byte(s)
		fmt.Printf("%d %q\n", i, s)
		fmt.Printf("  count %d %d valid %t %t full %t %t\n",
			utf8.RuneCountInString(s), utf8.RuneCount(b), utf8.ValidString(s), utf8.Valid(b),
			utf8.FullRuneInString(s), utf8.FullRune(b))
		r, n := utf8.DecodeRuneInString(s)
		r2, n2 := utf8.DecodeRune(b)
		fmt.Printf("  first %U %d %U %d\n", r, n, r2, n2)
		r, n = utf8.DecodeLastRuneInString(s)
		r2, n2 = utf8.DecodeLastRune(b)
		fmt.Printf("  last %U %d %U %d\n", r, n, r2, n2)
		var runes []rune
		for len(s) > 0 {
			r, n := utf8.DecodeRuneInString(s)
			runes = append(runes, r)
			s = s[n:]
		}
		fmt.Printf("  runes %v\n", runes)
		starts := 0
		for j := range b {
			if utf8.RuneStart(b[j]) {
				starts++
			}
		}
		fmt.Println("  starts", starts)
	}

	for _, r := range []rune{0, 'a', 0x7f, 0x80, 0x7ff, 0x800, 0xd7ff, 0xd800, 0xdfff, 0xe000, 0xffff, 0x10000,
		0x10ffff, 0x110000, -1} {
		buf := make([]byte, utf8.UTFMax)
		n := utf8.EncodeRune(buf, r)
		app := utf8.AppendRune([]byte("x"), r)
		fmt.Printf("rune %U len %d valid %t encoded %x appended %x\n", r, utf8.RuneLen(r), utf8.ValidRune(r), buf[:n], app)
	}

	fmt.Println(utf8.RuneError, utf8.RuneSelf, utf8.MaxRune, utf8.UTFMax)
}

// Output:
// 0 ""
//   count 0 0 valid true true full false false
//   first U+FFFD 0 U+FFFD 0
//   last U+FFFD 0 U+FFFD 0
//   runes []
//   starts 0
// 1 "a"
//   count 1 1 valid true true full true true
//   first U+0061 1 U+0061 1
//   last U+0061 1 U+0061 1
//   runes [97]
//   starts 1
// 2 "hello, world"
//   count 12 12 valid true true full true true
//   first U+0068 1 U+0068 1
//   last U+0064 1 U+0064 1
//   runes [104 101 108 108 111 44 32 119 111 114 108 100]
//   starts 12
// 3 "héllo"
//   count 5 5 valid true true full true true
//   first U+0068 1 U+0068 1
//   last U+006F 1 U+006F 1
//   runes [104 233 108 108 111]
//   starts 5
// 4 "世界"
//   count 2 2 valid true true full true true
//   first U+4E16 3 U+4E16 3
//   last U+754C 3 U+754C 3
//   runes [19990 30028]
//   starts 2
// 5 "😀 and \U0010ffff"
//   count 7 7 valid true true full true true
//   first U+1F600 4 U+1F600 4
//   last U+10FFFF 4 U+10FFFF 4
//   runes [128512 32 97 110 100 32 1114111]
//   starts 7
// 6 "\xff"
//   count 1 1 valid false false full true true
//   first U+FFFD 1 U+FFFD 1
//   last U+FFFD 1 U+FFFD 1
//   runes [65533]
//   starts 1
// 7 "abc\xffdef"
//   count 7 7 valid false false full true true
//   first U+0061 1 U+0061 1
//   last U+0066 1 U+0066 1
//   runes [97 98 99 65533 100 101 102]
//   starts 7
// 8 "\xc0\x80"
//   count 2 2 valid false false full true true
//   first U+FFFD 1 U+FFFD 1
//   last U+FFFD 1 U+FFFD 1
//   runes [65533 65533]
//   starts 1
// 9 "\xe0\x80\x80"
//   count 3 3 valid false false full true true
//   first U+FFFD 1 U+FFFD 1
//   last U+FFFD 1 U+FFFD 1
//   runes [65533 65533 65533]
//   starts 1
// 10 "\xed\xa0\x80"
//   count 3 3 valid false false full true true
//   first U+FFFD 1 U+FFFD 1
//   last U+FFFD 1 U+FFFD 1
//   runes [65533 65533 65533]
//   starts 1
// 11 "\xf4\x90\x80\x80"
//   count 4 4 valid false false full true true
//   first U+FFFD 1 U+FFFD 1
//   last U+FFFD 1 U+FFFD 1
//   runes [65533 65533 65533 65533]
//   starts 1
// 12 "\xe4\xb8"
//   count 2 2 valid false false full false false
//   first U+FFFD 1 U+FFFD 1
//   last U+FFFD 1 U+FFFD 1
//   runes [65533 65533]
//   starts 1
// 13 "\xf0\x9f\x98"
//   count 3 3 valid false false full false false
//   first U+FFFD 1 U+FFFD 1
//   last U+FFFD 1 U+FFFD 1
//   runes [65533 65533 65533]
//   starts 1
// 14 "\x80\x80"
//   count 2 2 valid false false full true true
//   first U+FFFD 1 U+FFFD 1
//   last U+FFFD 1 U+FFFD 1
//   runes [65533 65533]
//   starts 0
// 15 "a\xe4\xb8"
//   count 3 3 valid false false full true true
//   first U+0061 1 U+0061 1
//   last U+FFFD 1 U+FFFD 1
//   runes [97 65533 65533]
//   starts 2
// 16 "�"
//   count 1 1 valid true true full true true
//   first U+FFFD 3 U+FFFD 3
//   last U+FFFD 3 U+FFFD 3
//   runes [65533]
//   starts 1
// 17 "\x00\x7f\u0080߿ࠀ\uffff"
//   count 6 6 valid true true full true true
//   first U+0000 1 U+0000 1
//   last U+FFFF 3 U+FFFF 3
//   runes [0 127 128 2047 2048 65535]
//   starts 6
// rune U+0000 len 1 valid true encoded 00 appended 7800
// rune U+0061 len 1 valid true encoded 61 appended 7861
// rune U+007F len 1 valid true encoded 7f appended 787f
// rune U+0080 len 2 valid true encoded c280 appended 78c280
// rune U+07FF len 2 valid true encoded dfbf appended 78dfbf
// rune U+0800 len 3 valid true encoded e0a080 appended 78e0a080
// rune U+D7FF len 3 valid true encoded ed9fbf appended 78ed9fbf
// rune U+D800 len -1 valid false encoded efbfbd appended 78efbfbd
// rune U+DFFF len -1 valid false encoded efbfbd appended 78efbfbd
// rune U+E000 len 3 valid true encoded ee8080 appended 78ee8080
// rune U+FFFF len 3 valid true encoded efbfbf appended 78efbfbf
// rune U+10000 len 4 valid true encoded f0908080 appended 78f0908080
// rune U+10FFFF len 4 valid true encoded f48fbfbf appended 78f48fbfbf
// rune U+110000 len -1 valid false encoded efbfbd appended 78efbfbd
// rune U+FFFFFFFFFFFFFFFF len -1 valid false encoded efbfbd appended 78efbfbd
// 65533 128 1114111 4
