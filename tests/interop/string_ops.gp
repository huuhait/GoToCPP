// phase: 04

// A std::string keeps C++'s operators: += and + with string literals (an untyped constant next to
// a C++ value is a C literal), ==, <, and its members (size, substr, find, npos).
package main

import (
	"fmt"

	"<string>"
)

func main() {
	s := std.string("hello")
	s += ", world"
	s += '!' // a C char
	t := s + "?"
	fmt.Println(int(s.size()), int(t.size()), bool(s == "hello, world!"), bool(t != s))
	fmt.Println(bool(s < "help"), bool("a" < s), bool(s.substr(0, 5) == "hello"))
	at := s.find("world")
	if at != std.string.npos {
		fmt.Println("world at", int(at))
	}
	if s.find("moon") == std.string.npos {
		fmt.Println("no moon")
	}
	if !s.empty() && s[0] == 'h' {
		fmt.Println("first byte", int(s[0]))
	}
	s[0] = 'H'
	fmt.Println(bool(s.substr(0, 5) == "Hello"), bool(s.starts_with("Hell")))
	u := s.substr(7)
	u.append("s").append("!")
	fmt.Println(int(u.length()), bool(u == "world!s!"))
}

// Output:
// 13 14 true true
// true true true
// world at 7
// no moon
// first byte 104
// true true
// 8 true
