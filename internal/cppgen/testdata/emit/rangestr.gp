// Range over a string (Phase 03 step 3.4): every form of the loop decodes UTF-8 like
// Go (an invalid byte is U+FFFD and steps one byte), the string is evaluated once,
// and the range variables are copies of hidden counters (Go 1.22). string(r) of an
// integer is the UTF-8 of the rune, or "�" for a value that is no code point.
package main

import "fmt"

var evals = 0

func text() string {
	evals++
	return "hé\xff世"
}

func main() {
	s := "aé世\U0001F600"
	for i, r := range s {
		fmt.Println(i, r, string(r))
	}
	for i := range s {
		print(i, " ")
	}
	println()
	n := 0
	for range s {
		n++
	}
	for _, r := range "x\xc0\x80y\xe2\x82" {
		print(r, " ")
	}
	println(n)

	// Assigning to the variables, or to s, doesn't change the iteration.
	for i, r := range s {
		i += 10
		r = 'z'
		s = "changed"
		fmt.Println(i, r)
	}
	fmt.Println(s)

	// The string is evaluated once; variables that exist are assigned.
	var at int
	var last rune
	for at, last = range text() {
		if last == 0xFFFD {
			continue
		}
		fmt.Println("at", at, last)
	}
	fmt.Println(at, last, evals)
	for i, s := range s {
		fmt.Println(i, s) // s is the rune here
	}

	// string(r): the edge code points, from integers of several types.
	var neg, sur, big rune = -1, 0xD800, 0x110000
	var max, low rune = 0x10FFFF, 0xDFFF
	var b byte = 0xE9
	var u uint64 = 1<<64 - 1
	var wide int64 = 0x100000041
	var u16 uint16 = 0xD7FF
	fmt.Println(string(neg), string(sur), string(big), len(string(max)), string(low) == "\uFFFD")
	fmt.Println(string(b), string(u), string(wide), len(string(u16)), string(b)+string(rune(b+1)))
	for _, r := range string(max) + string(big) {
		print(r, " ")
	}
	println()
}
