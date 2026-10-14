// phase: 02

// Strings: concatenation in a loop, comparison, byte indexing (a byte
// prints as a number), len (in bytes, also for non-ASCII text) and
// escapes in literals.
package main

import "fmt"

func repeat(s string, n int) string {
	out := ""
	for range n {
		out += s
	}
	return out
}

func countByte(s string, b byte) int {
	n := 0
	for i := 0; i < len(s); i++ {
		if s[i] == b {
			n++
		}
	}
	return n
}

func reverseCompare(a, b string) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

func main() {
	s := "hello"
	t := s + ", " + "world"
	fmt.Println(t, len(t))
	fmt.Println(repeat("ab", 3), len(repeat("xyz", 10)))
	fmt.Println(s[0], s[4], s[1] == 'e', s[0]+1)
	fmt.Println(countByte("banana", 'a'), countByte("", 'a'))
	fmt.Println("abc" < "abd", "abc" < "ab", "" < "a", "Z" < "a", "b" > "abc")
	fmt.Println(reverseCompare("x", "y"), reverseCompare("y", "x"), reverseCompare("q", "q"))
	u := "héllo, 世界"
	fmt.Println(u, len(u), u[1], u[2])
	esc := "tab\there\nnewline \"quoted\" back\\slash \x41\101\u00e9"
	fmt.Println(esc, len(esc))
	raw := `raw \n string`
	fmt.Println(raw, len(raw))
	var empty string
	fmt.Println(empty == "", len(empty), empty+"x")
	same := "go" + "pher"
	fmt.Println(same == "gopher", same != "gopher")
	acc := ""
	for i := range 5 {
		if i > 0 {
			acc += "-"
		}
		acc += "x"
	}
	fmt.Println(acc)
}

// Output:
// hello, world 12
// ababab 30
// 104 111 true 105
// 3 0
// true false true true true
// -1 1 0
// héllo, 世界 14 195 169
// tab	here
// newline "quoted" back\slash AAé 41
// raw \n string 13
// true 0 x
// true false
// x-x-x-x-x
