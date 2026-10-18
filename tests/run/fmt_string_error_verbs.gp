// phase: 05

// Verbs that don't fit a string print Go's error form %!d(string=x), also inside composite
// values.
package main

import "fmt"

type S string

func main() {
	fmt.Printf("%d %t %c %f %e %b %o %U %p\n", "x", "x", "x", "x", "x", "x", "x", "x", "x")
	fmt.Printf("%d %s\n", S("x"), S("y"))
	fmt.Printf("%d %d\n", []string{"a"}, map[string]string{"k": "v"})
	fmt.Printf("%z %!\n", "x")
}

// Output:
// %!d(string=x) %!t(string=x) %!c(string=x) %!f(string=x) %!e(string=x) %!b(string=x) %!o(string=x) %!U(string=x) %!p(string=x)
// %!d(main.S=x) y
// [%!d(string=a)] map[%!d(string=k):%!d(string=v)]
// %!z(string=x) %!!(MISSING)
