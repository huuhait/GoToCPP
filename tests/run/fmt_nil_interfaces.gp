// phase: 05

// A nil interface where a function of fmt takes one: fmt.FormatString(nil, 'v') and Fprintf(nil,
// ...) panic with a nil dereference; fmt.State, Stringer, GoStringer and Formatter variables are
// nil interfaces that compare equal to nil and can be assigned; a nil Stringer is <nil> to
// Println.
package main

import "fmt"

func use(s fmt.Stringer, g fmt.GoStringer, f fmt.Formatter, st fmt.State) string {
	return fmt.Sprint(s == nil, g == nil, f == nil, st == nil, s, g, f, st)
}

func main() {
	var s fmt.Stringer
	var st fmt.State
	fmt.Println(use(nil, nil, nil, nil), use(s, nil, nil, st))
	fmt.Printf("%v %T %d\n", s, s, st)
	defer func() { fmt.Println("recovered:", recover()) }()
	fmt.Println(fmt.FormatString(nil, 'v'))
}

// Output:
// true true true true <nil> <nil> <nil> <nil> true true true true <nil> <nil> <nil> <nil>
// <nil> <nil> %!d(<nil>)
// recovered: runtime error: invalid memory address or nil pointer dereference
