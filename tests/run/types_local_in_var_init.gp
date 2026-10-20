// phase: 05

// A type declared inside a func literal in the initial value of a package variable, with a name
// that a package-level type has too (the local type hides it).
package main

import "fmt"

type P int

type Case struct {
	name string
	fn   func() string
}

var cases = []Case{
	{"shadow", func() string {
		type P struct{ x, y int }
		return fmt.Sprintf("%v %+v %T", P{1, 2}, P{3, 4}, P{})
	}},
	{"other", func() string {
		type Q struct{ s []string }
		type R map[string]Q
		return fmt.Sprintf("%#v", R{"k": {[]string{"a"}}})
	}},
}

var total = func() int {
	type acc struct{ n int }
	a := acc{}
	for i := range 4 {
		a.n += i
	}
	return a.n
}()

func main() {
	for _, c := range cases {
		fmt.Println(c.name, c.fn())
	}
	fmt.Println(P(3), total)
}

// Output:
// shadow {1 2} {x:3 y:4} main.P
// other main.R{"k":main.Q{s:[]string{"a"}}}
// 3 6
