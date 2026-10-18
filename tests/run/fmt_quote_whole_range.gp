// phase: 05

// %q and %+q of every code point from -2 to U+110001 (as runes), hashed with FNV-1a in the
// program, so that an error in the table of printable runes shows (it is checked against Go's
// output); invalid strings too.
package main

import "fmt"

type fnv uint64

func (h *fnv) add(s string) {
	for i := 0; i < len(s); i++ {
		*h = (*h ^ fnv(s[i])) * 1099511628211
	}
}

func main() {
	h := fnv(14695981039346656037)
	n := 0
	for r := rune(-2); r <= 0x110001; r++ {
		h.add(fmt.Sprintf("%q", r))
		if r < 0x2000 || r&0x7f == 0 {
			h.add(fmt.Sprintf("%+q%q%+q", r, string(r), string(r)))
		}
		n++
	}
	fmt.Println(n, uint64(h))
	h = fnv(14695981039346656037)
	for b := 0; b < 256; b++ {
		for c := 0; c < 256; c += 15 {
			s := string([]byte{byte(b), byte(c), 'x'})
			h.add(fmt.Sprintf("%q%+q", s, s))
		}
	}
	fmt.Println(uint64(h))
}

// Output:
// 1114116 10622851085910190723
// 14362601930323127370
