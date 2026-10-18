// phase: 09

// unicode/utf8 (ported): every code point encodes and decodes back, the three ways of encoding
// agree with each other and with the conversion string(rune), and a loop over every byte pair finds
// the same valid/invalid split as Go's.
package main

import (
	"fmt"
	"unicode/utf8"
)

func main() {
	buf := make([]byte, utf8.UTFMax)
	var bad, sumLen, strMismatch, appendMismatch int
	for r := rune(0); r <= utf8.MaxRune; r++ {
		valid := utf8.ValidRune(r)
		n := utf8.EncodeRune(buf, r)
		got, size := utf8.DecodeRune(buf[:n])
		if n != size || n != utf8.RuneLen(r) && valid {
			bad++
		}
		if valid && got != r {
			bad++
		}
		if !valid && (got != utf8.RuneError || n != 3) {
			bad++
		}
		if string(buf[:n]) != string(r) {
			strMismatch++
		}
		if string(utf8.AppendRune(nil, r)) != string(buf[:n]) {
			appendMismatch++
		}
		sumLen += n
	}
	fmt.Println("bad", bad, "string mismatch", strMismatch, "append mismatch", appendMismatch)
	fmt.Println("total bytes", sumLen)

	// every two-byte string: how many are valid, and how many runes the invalid ones count as
	var valid, runes int
	two := make([]byte, 2)
	for a := 0; a < 256; a++ {
		for b := 0; b < 256; b++ {
			two[0], two[1] = byte(a), byte(b)
			if utf8.Valid(two) {
				valid++
			}
			runes += utf8.RuneCount(two)
		}
	}
	fmt.Println("valid pairs", valid, "runes", runes)

	// a long string: count, decode backwards, compare
	s := ""
	for i := 0; i < 2000; i++ {
		s += string(rune(i*37%0x2000 + 0x20))
	}
	fmt.Println(len(s), utf8.RuneCountInString(s), utf8.ValidString(s))
	back := 0
	for t := s; len(t) > 0; {
		_, n := utf8.DecodeLastRuneInString(t)
		t = t[:len(t)-n]
		back++
	}
	fmt.Println("backwards", back)
}

// Output:
// bad 0 string mismatch 0 append mismatch 0
// total bytes 4388736
// valid pairs 18304 runes 129152
// 5477 2000 true
// backwards 2000
