// phase: 04

// <regex>: regex_match, regex_search with smatch (match groups), regex_replace, and
// sregex_iterator over a string with several matches.
package main

import (
	"fmt"

	"<regex>"
	"<string>"
)

func main() {
	re := std.regex(`(\d+)-(\w+)`)
	fmt.Println(std.regex_match("123-abc", re), std.regex_match("123", re))

	text := std.string("a1-xx b22-yy c333-zz")
	var m std.smatch
	if std.regex_search(text, m, re) {
		fmt.Println(string(m[0].str()), string(m[1].str()), string(m[2].str()), int(m.size()))
	}
	fmt.Println(string(std.regex_replace(text, re, "<$2:$1>")))

	n := 0
	for it := std.sregex_iterator(text.begin(), text.end(), re); it != std.sregex_iterator(); it++ {
		n += int((*it)[1].length())
	}
	fmt.Println(n)
}

// Output:
// true false
// 1-xx 1 xx 3
// a<xx:1> b<yy:22> c<zz:333>
// 6
