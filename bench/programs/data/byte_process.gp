// bench: byte_process
// category: data
// size: 20000000
// smoke: 20000
// twin: yes
// about: byte loops over generated text: word and line counts, upper-casing, byte histogram, hash
package main

import "fmt"

var seed uint64 = 4242

func rnd() uint64 {
	seed = seed*6364136223846793005 + 1442695040888963407
	return seed >> 33
}

func isLetter(c byte) bool { return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' }

func main() {
	n := benchN
	data := make([]byte, n)
	for i := range data {
		r := rnd() % 32
		switch {
		case r < 24:
			data[i] = byte('a' + r)
		case r < 29:
			data[i] = ' '
		case r < 30:
			data[i] = '\n'
		default:
			data[i] = byte('A' + r - 30)
		}
	}

	words, lines := 0, 0
	inWord := false
	for _, c := range data {
		if c == '\n' {
			lines++
		}
		if isLetter(c) {
			if !inWord {
				words++
				inWord = true
			}
		} else {
			inWord = false
		}
	}

	upper := make([]byte, n)
	for i, c := range data {
		if c >= 'a' && c <= 'z' {
			c -= 'a' - 'A'
		}
		upper[i] = c
	}

	var hist [256]int
	for _, c := range upper {
		hist[c]++
	}

	var table [256]byte
	for i := range table {
		table[i] = byte(i*7 + 3)
	}
	h := uint64(14695981039346656037)
	for _, c := range upper {
		h = (h ^ uint64(table[c])) * 1099511628211
	}

	s := string(data[:n/4])
	vowels := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case 'a', 'e', 'i', 'o', 'u':
			vowels++
		}
	}
	fmt.Println(words, lines, hist['A'], hist['B'], hist[' '], h, vowels)
}
