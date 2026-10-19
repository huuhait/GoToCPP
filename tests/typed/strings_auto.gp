// phase: 12
// opaque: reject

// A gocpp string goes to a const char* or a std::string parameter by itself: gp::cstr's NUL-terminated
// copy (or the bytes, when a NUL follows them), alive until the end of the statement. A set of
// overloads of different kinds is left to C++.
package main

import (
	"fmt"

	"<cstdio>"
	"<cstdlib>"
	"<cstring>"
	"<string>"
	"./strs.hpp"
)

func build(n int) string {
	s := "x"
	for i := 0; i < n; i++ {
		s += "y"
	}
	return s
}

func main() {
	s := "banana"
	fmt.Println(strs.count_char(s, 'a'), strs.count_char(s+"!", 'n'))

	// A const char* parameter, a substring (not NUL-terminated), a call's result.
	fmt.Println(int(cstring.strlen(s)), int(cstring.strlen(s[1:4])), int(cstring.strlen(build(4))))
	fmt.Println(cstring.strcmp(s, "banana"), cstring.strcmp(s, "apple") > 0, cstring.strcmp(s[:3], "ban"))
	fmt.Println(int(cstdlib.atoi(build(0)+"41")) + 1)

	// A std::string parameter, and a set of overloads for string and wstring.
	fmt.Println(strs.shout(s), strs.total_len(s, "xy"), strs.length_sv(s))
	fmt.Println(std.stoi(s[:0] + "123"))

	// A mixed overload set is C++'s: a gocpp string converts to the string_view only.
	fmt.Println(C.GoString(strs.kind(s)))

	// A variadic argument: %s takes a const char*.
	var buf [32]C.char
	n := cstdio.snprintf(&buf[0], 32, "%s-%d-%s", s, 7, build(1))
	fmt.Println(n, C.GoString(&buf[0]))

	// A method of a class that does not keep pointers.
	var meter strs.Meter
	meter.count(s)
	fmt.Println(meter.count(s + "!"))

	// A pointer into the string that is used in the statement is fine.
	if cstring.strstr(s, "nan") != nil {
		fmt.Println("found", C.GoString(cstring.strstr(s, "nan")))
	}

	// A NUL byte ends the C string, as with C.CString: the string itself is 5 bytes.
	t := "ab\x00cd"
	fmt.Println(int(cstring.strlen(t)), len(t))
}

// Output:
// 3 2
// 6 3 5
// 0 true 0
// 1
// BANANA! 8 6
// 123
// view
// 11 banana-7-xy
// 13
// found nana
// 2 5
