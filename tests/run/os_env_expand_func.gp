// phase: 09

package main

import (
	"fmt"
	"os"
)

func main() {
	m := map[string]string{"a": "AAA", "b": "B", "": "EMPTYNAME", "1": "ONE", "*": "STAR", "foo": "bar"}
	f := func(k string) string { return "<" + m[k] + ">" }
	for _, s := range []string{"$a $b", "${a}x", "$$a", "$1", "${1}", "$*", "$", "${", "${}", "$foo.bar", "$foo-$foo", "$ x", "a$", "$#", "${a b}", "$a$b", "\u00e9$a"} {
		fmt.Printf("%q -> %q\n", s, os.Expand(s, f))
	}
	var seen []string
	os.Expand("$x ${y} $$ $1 ${z}", func(k string) string { seen = append(seen, k); return "" })
	fmt.Printf("%q\n", seen)
	fmt.Printf("%q\n", os.Expand("none", func(k string) string { panic("called") }))
}

// Output:
// "$a $b" -> "<AAA> <B>"
// "${a}x" -> "<AAA>x"
// "$$a" -> "<>a"
// "$1" -> "<ONE>"
// "${1}" -> "<ONE>"
// "$*" -> "<STAR>"
// "$" -> "$"
// "${" -> ""
// "${}" -> ""
// "$foo.bar" -> "<bar>.bar"
// "$foo-$foo" -> "<bar>-<bar>"
// "$ x" -> "$ x"
// "a$" -> "a$"
// "$#" -> "<>"
// "${a b}" -> "<>"
// "$a$b" -> "<AAA><B>"
// "é$a" -> "é<AAA>"
// ["x" "y" "$" "1" "z"]
// "none"
