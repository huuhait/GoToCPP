// phase: 03

// Substrings share the bytes of their string: every form of s[a:b], bounds of
// several integer types, substrings of substrings and of literals, and empty ones.
package main

import "fmt"

func main() {
	s := "Hello, 世界!"
	fmt.Println(len(s), s[7:], s[:5], s[7:10], s[:], s[len(s):], len(s[3:3]))
	var lo uint8 = 1
	var hi int16 = 5
	var w uint64 = 2
	var n int32 = 4
	fmt.Println(s[lo:hi], s[lo:], s[:hi], s[w:n], s[n:][:w], s[lo:hi][1:])
	fmt.Println("literal"[3:], "literal"[:3], "x"[1:] == "", s[7:10] == "世")
	for i := 0; i <= len("abc"); i++ {
		fmt.Println(i, "abc"[i:], "abc"[:i], "["+"abc"[i:i]+"]")
	}
	t := s
	s = "changed"
	fmt.Println(t[:5], s[:3])
	// A substring keeps its bytes when the string it came from is gone.
	sub := (s + "!!" + s)[5:10]
	fmt.Println(sub, len(sub))
}
