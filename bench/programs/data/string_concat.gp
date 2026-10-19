// bench: string_concat
// category: data
// size: 20000
// smoke: 50
// twin: no
// about: s += small, []byte appends turned into strings, integer to string and back
package main

import "fmt"

func parse(s string) int {
	n := 0
	for i := 0; i < len(s); i++ {
		n = n*10 + int(s[i]-'0')
	}
	return n
}

func main() {
	n := benchN
	var total uint64

	// s += small: strings of 200 bytes built piece by piece
	for r := 0; r < n; r++ {
		s := ""
		for j := 0; j < 100; j++ {
			s += "ab"
		}
		total += uint64(len(s)) + uint64(s[r%len(s)])
	}

	// byte buffers appended to, converted to a string once
	for r := 0; r < n; r++ {
		var b []byte
		for j := 0; j < 100; j++ {
			b = append(b, 'x', byte('a'+j%26))
		}
		s := string(b)
		total += uint64(len(s)) + uint64(s[(r*3)%len(s)])
	}

	// integer to string and back
	for r := 0; r < n*10; r++ {
		s := fmt.Sprint(r)
		total += uint64(parse(s)) + uint64(len(s))
	}

	// concatenating three strings, comparing
	eq := 0
	for r := 0; r < n*10; r++ {
		a := "key-" + fmt.Sprint(r&255) + "-end"
		if a == "key-7-end" || a < "key-1" {
			eq++
		}
	}
	fmt.Println(total, eq)
}
