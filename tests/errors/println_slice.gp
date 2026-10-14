// phase: 03

// print and println of a slice are errors (D23): Go prints the length, the capacity and an
// address of its heap there, [3/3]0xc000012345, which gocpp can't print the same.
package main

import "fmt"

func main() {
	s := []int{1, 2, 3}
	println(len(s), s)    // ERROR "cannot println s \(\[\]int\): println prints a slice as \[len/cap\]0x<address>, which gocpp can't print; use fmt.Println"
	print([]byte("x"), 1) // ERROR "cannot print \[\]byte\(\x22x\x22\) \(\[\]byte\): print prints a slice as"
	println(s[0], len(s))
	fmt.Println(len(s))
}
