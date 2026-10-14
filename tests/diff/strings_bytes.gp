// phase: 02

// Strings are bytes: indexing gives a byte (it prints as a number), len
// counts bytes, comparison is by unsigned bytes (so non-ASCII sorts after
// ASCII), and concatenation keeps every byte, also zero bytes.
package main

import "fmt"

func cmp(a, b string) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

func main() {
	a := "é"
	b := "z"
	c := "\xff"
	d := "\x00"
	e := ""
	fmt.Println(cmp(a, b), cmp(b, a), cmp(c, a), cmp(d, e), cmp(e, d), cmp(a, a))
	fmt.Println(a < b, c > b, d > e, "a\x00" > "a", "a\x00b" < "a\x01", "日本" < "日本語", "日" > "z")
	fmt.Println(len(a), len(c), len(d), len("日本語"), a[0], a[1], c[0], d[0])
	s := "x" + d + "y" + c
	fmt.Println(len(s), s[0], s[1], s[2], s[3])
	fmt.Println(s == "x\x00y\xff", s != "x\x00y", s > "x", s < "x\x01")
	t := "日本語"
	sum := 0
	for i := 0; i < len(t); i++ {
		sum += int(t[i])
		fmt.Print(t[i], ",")
	}
	fmt.Println(sum)
	fmt.Println(t, t+t, len(t+t))
	h := "hello"
	fmt.Println(h[0]-'a', h[len(h)-1], h[1] < h[2], h[2] == h[3], h[4]+h[4])
	n := ""
	for i := range 5 {
		n = n + h
		fmt.Println(len(n), n[i])
	}
	fmt.Println("a"+"b"+"c" == "abc", "ab"+"c" < "abd")
	fmt.Println("tab\tcr\rbell\a esc\x1b del\x7f nul\x00 end")
}
