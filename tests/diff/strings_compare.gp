// phase: 02

// String comparison operators on a grid of strings: equal prefixes,
// different lengths, empty strings, upper and lower case, digits and bytes
// above 0x7f.
package main

import "fmt"

func str(i int) string {
	switch i {
	case 0:
		return ""
	case 1:
		return "a"
	case 2:
		return "A"
	case 3:
		return "ab"
	case 4:
		return "abc"
	case 5:
		return "abd"
	case 6:
		return "b"
	case 7:
		return "10"
	case 8:
		return "9"
	case 9:
		return "\xc3\xa9"
	case 10:
		return "\x7f"
	case 11:
		return "\x80"
	}
	return "a\x00"
}

func main() {
	const n = 13
	for i := range n {
		for j := range n {
			a, b := str(i), str(j)
			fmt.Println(i, j, a == b, a != b, a < b, a <= b, a > b, a >= b, len(a+b))
		}
	}
}
