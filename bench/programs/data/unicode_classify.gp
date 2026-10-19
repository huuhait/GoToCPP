// bench: unicode_classify
// category: data
// size: 30000000
// smoke: 30000
// twin: no
// about: unicode.IsLetter, IsDigit, IsSpace, IsUpper, ToUpper and ToLower over generated runes (ASCII, Latin-1 and CJK)
package main

import (
	"fmt"
	"unicode"
)

func main() {
	var letters, digits, spaces, uppers int
	var sum uint64
	seed := uint64(7)
	for i := 0; i < benchN; i++ {
		seed = seed*6364136223846793005 + 1442695040888963407
		var r rune
		switch (seed >> 60) & 3 {
		case 0:
			r = rune(seed>>33) % 128
		case 1:
			r = 0xC0 + rune(seed>>33)%0x100
		case 2:
			r = 0x4E00 + rune(seed>>33)%0x5000
		default:
			r = rune(seed>>33) % 0x3000
		}
		if unicode.IsLetter(r) {
			letters++
		}
		if unicode.IsDigit(r) {
			digits++
		}
		if unicode.IsSpace(r) {
			spaces++
		}
		if unicode.IsUpper(r) {
			uppers++
		}
		sum += uint64(unicode.ToUpper(r)) + uint64(unicode.ToLower(r))
	}
	fmt.Println(letters, digits, spaces, uppers, sum)
}
