// phase: 12
// opaque: reject
// windows: yes

// Only headers of the C++ standard library, so that the program is typed for Windows too (LLP64: long
// is 32 bits and size_t is unsigned long long): members through smart pointers, a reference
// result, C++'s conversions into gocpp numbers, a gocpp string for a const char* and a std::string.
package main

import (
	"fmt"

	"<cstdlib>"
	"<cstring>"
	"<map>"
	"<memory>"
	"<optional>"
	"<string>"
	"<vector>"
)

func main() {
	up := std.make_unique[std.vector[int]]()
	up.push_back(4)
	up.push_back(9)
	fmt.Println(up.size(), up.back(), up.get().size())

	opt := std.optional[std.string](std.string("abc"))
	fmt.Println(opt.has_value(), opt.size(), opt.length())

	v := std.vector[int]{10, 20, 30}
	v.at(1) = 21
	p := &v.back()
	*p += 5
	fmt.Println(v.at(0), v.at(1), v.at(2), *p)

	word := "windows" + fmt.Sprint(2)
	var n int = cstring.strlen(word)
	var l int = cstdlib.labs(-5)
	total := cstring.strlen("abc") + len(word)
	fmt.Println(n, l, int(total), int(cstdlib.atoi(word[7:])))

	counts := std.map[std.string, int]{}
	for _, w := range []string{"x", "y", "x"} {
		counts[w]++
	}
	fmt.Println(counts.size(), counts["x"], counts["y"])
}

// Output:
// 2 9 2
// true 3 3
// 10 21 35 35
// 8 5 11 2
// 2 2 1
