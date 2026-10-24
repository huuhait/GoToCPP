// phase: 05

// Conversions between strings and slices whose elements are named byte and rune types: string to
// []MyByte and back, []R and string both ways, a named slice type of them, and the empty and nil
// cases (gocpp once said "not supported yet": internal/cppgen TestUnsupported listed them).
package main

import "fmt"

type MyByte byte
type R rune
type Bs []MyByte
type Rs []R

func main() {
	s := "héllo"
	b := []MyByte(s)
	fmt.Println(len(b), b[1], string(b) == s)
	b[0] = 'H'
	fmt.Println(string(b), s)
	r := []R(s)
	fmt.Println(len(r), r[1], string(r))
	fmt.Println(string([]R{'a', 'é', 0x1F600}), string([]MyByte{'o', 'k'}))
	var bs Bs = Bs("xyz")
	var rs Rs = Rs("αβ")
	fmt.Println(string(bs), len(rs), string(rs))
	var nb []MyByte
	var nr []R
	fmt.Printf("%q %q %d %d\n", string(nb), string(nr), len([]MyByte("")), len([]R("")))
	fmt.Println(string([]R{-1, 0x110000}) == "��")
}

// Output:
// 6 195 true
// Héllo héllo
// 5 233 héllo
// aé😀 ok
// xyz 2 αβ
// "" "" 0 0
// true
