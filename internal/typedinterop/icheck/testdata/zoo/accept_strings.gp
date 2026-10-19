// Strings: a literal goes to a char* and to a std::string, C.cstr makes a char* of a Go string,
// std.string(s) a std::string, and a Go string converts to a string_view.
package main

import (
	"./shapes.hpp"
	"<cstdio>"
	"<cstring>"
	"<string>"
	"fmt"
)

func main() {
	s := "hello"
	n := cstring.strlen(C.cstr(s))
	cstdio.puts(C.cstr(s))
	var cs std.string = std.string(s)
	fmt.Println(n, cs.size(), shapes.takes_str(cs), shapes.takes_str("lit"), shapes.takes_cstr(C.cstr(s)))
}
