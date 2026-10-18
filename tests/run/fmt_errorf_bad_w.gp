// phase: 05

// %w is only for Errorf and only for errors: Sprintf, Printf and Errorf with a non-error operand
// print the error form %!w(...); the other verbs work in Errorf as in Sprintf.
package main

import "fmt"

type E struct{}

func (E) Error() string { return "E" }

func main() {
	fmt.Println(fmt.Sprintf("%w", E{}))
	fmt.Printf("%w|%w\n", E{}, 3)
	e := fmt.Errorf("a %w b", 5)
	fmt.Println(e)
	fmt.Printf("%T\n", e)
	e = fmt.Errorf("%w %w", E{}, "str")
	fmt.Println(e)
	fmt.Printf("%T\n", e)
	e = fmt.Errorf("%v %+v %#v %s %d %x %q %10v|%-6s|", E{}, E{}, E{}, E{}, E{}, E{}, E{}, E{}, E{})
	fmt.Println(e)
	e = fmt.Errorf("%d items, %5.2f%%, %v", 3, 99.5, []int{1})
	fmt.Println(e)
	e = fmt.Errorf("%d %d", 1)
	fmt.Println(e)
	e = fmt.Errorf("x", 1, "y")
	fmt.Println(e)
}

// Output:
// %!w(main.E={})
// %!w(main.E={})|%!w(int=3)
// a %!w(int=5) b
// *fmt.wrapError
// E %!w(string=str)
// *fmt.wrapErrors
// E E main.E{} E {} 45 "E"          E|E     |
// 3 items, 99.50%, [1]
// 1 %!d(MISSING)
// x%!(EXTRA int=1, string=y)
