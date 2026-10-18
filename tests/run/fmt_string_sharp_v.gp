// phase: 05

// %#v of strings quotes them with Go syntax; %+q escapes non-ASCII; %#q uses backquotes when it
// can.
package main

import "fmt"

type Name string

func main() {
	fmt.Printf("%#v %#v %#v %#v\n", "hi", "tab\there", "é😀", Name("n"))
	fmt.Printf("%+q %+q %q %#q %#q %#q\n", "日本", "é", "日本", "raw `string", "plain", "tab\tin")
	fmt.Printf("%#v %v %q\n", []string{"a", "b"}, []string{"a b", "c"}, []string{"a", "b"})
	fmt.Printf("%#v\n", map[string]string{"k": "v"})
	fmt.Printf("%5.2q|%-8q|%08q|\n", "abcdef", "ab", "ab")
}

// Output:
// "hi" "tab\there" "é😀" "n"
// "\u65e5\u672c" "\u00e9" "日本" "raw `string" `plain` `tab	in`
// []string{"a", "b"} [a b c] ["a" "b"]
// map[string]string{"k":"v"}
//  "ab"|"ab"    |0000"ab"|
