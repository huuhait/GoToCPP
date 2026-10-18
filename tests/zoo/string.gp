// phase: 04

// <string>: std::string from a Go string (explicit), find/substr/append/compare, npos (a static
// member: `::`), conversion back with string(s), and std::to_string / std::stoi.
package main

import (
	"fmt"

	"<string>"
)

func main() {
	s := std.string("hello, world")
	i := s.find("world")
	fmt.Println(int(i), i != std.string.npos, s.find("zzz") == std.string.npos)

	sub := s.substr(7, 5)
	sub.append("!")
	fmt.Println(string(sub), int(sub.size()), int(s.compare("hello")) > 0)

	goStr := "gopher"
	t := std.string(goStr)
	t += "s"
	t[0] = 'G'
	fmt.Println(string(t), int(t.length()))

	n := std.stoi("1234")
	fmt.Println(int(n)+1, string(std.to_string(77)), string(std.to_string(1.5))[:3])
	fmt.Println(s.starts_with("hello"), s.ends_with("x"), s.find("lo, w") != std.string.npos)
}

// Output:
// 7 true true
// world! 6 true
// Gophers 7
// 1235 77 1.5
// true false true
