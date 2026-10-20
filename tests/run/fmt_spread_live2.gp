// phase: 09

// More of the live operands of f(a...) (the general verb path, argument indexes, Errorf), FormatString as a
// func value, and array and struct literals in package variables of interface type.
package main

import "fmt"

var shared []any

type Mut struct{ tag string }

func (m *Mut) String() string {
	shared[1] = "changed by " + m.tag
	shared[2] = 99
	return "mut-" + m.tag
}

func fresh() {
	shared = []any{&Mut{"a"}, "orig", 1, "tail"}
}

type F struct{}

func (F) Format(s fmt.State, c rune) {
	fs := fmt.FormatString
	fmt.Fprint(s, fs(s, c))
}

var arrAny any = [3]int{1, 2, 3}
var arrStr fmt.Stringer = nil
var structAny any = struct{ A, B int }{1, 2}

func main() {
	// P09: the general verb path (width, flags) reads the operands live too.
	fresh()
	fmt.Printf("%v|%8v|%-4d|%s\n", shared...)
	// P11: Errorf(format, a...) reads them live.
	fresh()
	fmt.Println(fmt.Errorf("%v|%v|%d|%s", shared...))
	// argument indexes.
	fresh()
	fmt.Printf("%[1]v|%[2]v|%[3]d\n", shared...)
	// L07: FormatString as a func value.
	fmt.Printf("%-+8.3x|%v\n", F{}, F{})
	// L12: an array literal in an any package variable.
	fmt.Println(arrAny, arrStr, structAny)
}

// Output:
// mut-a|changed by a|99  |tail
// mut-a|changed by a|99|tail
// mut-a|changed by a|99
// %+-8.3x|%v
// [1 2 3] <nil> {1 2}
