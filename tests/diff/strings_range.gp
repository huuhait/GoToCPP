// phase: 03

// Range over a string decodes UTF-8 like Go: the index of each rune's first byte,
// U+FFFD for each invalid byte (one byte at a time), every loop form, and the
// range variables as copies (Go 1.22): changing them or the string in the body
// changes nothing.
package main

import "fmt"

func show(s string) {
	for i, r := range s {
		fmt.Print(i, ":", r, " ")
	}
	fmt.Println("len", len(s))
}

var calls = 0

func text() string {
	calls++
	return "a\xffé"
}

func main() {
	show("")
	show("plain ascii")
	show("héllo, 世界 \U0001F600")
	show("\x80\xbf\xc0\x80\xc1\xbf\xf5\x80\xff")
	show("\xc2\x80\xdf\xbf\xe0\xa0\x80\xef\xbf\xbf\xf0\x90\x80\x80\xf4\x8f\xbf\xbf")
	show("\xe0\x9f\xbf \xed\xa0\x80 \xed\xbf\xbf \xf0\x8f\xbf\xbf \xf4\x90\x80\x80")
	show("\xc2 \xe2\x82 \xf0\x9f\x98 \xc2\x41 \xe2\x41\xac \xf0\x9f\x98\x41")
	show("a\x00b")
	show("\xef\xbf\xbd")

	n := 0
	for range "日本語" {
		n++
	}
	for i := range "日本語" {
		fmt.Print(i, " ")
	}
	for _, r := range "日本語" {
		fmt.Print(r, " ")
	}
	fmt.Println(n)

	s := "añb"
	for i, r := range s {
		i *= 100
		r++
		s = ""
		fmt.Println(i, r, len(s))
	}
	var at int
	var last rune
	for at, last = range text() {
	}
	fmt.Println(at, last, calls)
	for at = range text() {
	}
	fmt.Println(at, calls)
	for i, s := range "xy" {
		fmt.Println(i, s)
	}
}
