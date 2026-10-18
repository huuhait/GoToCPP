// phase: 05

// Print adds a space between operands when neither is a string; Println always adds spaces and a
// newline; Sprint, Sprintln, Print with no operands and with nil.
package main

import "fmt"

type S string

type E struct{}

func (E) Error() string { return "err" }

type St struct{}

func (St) String() string { return "st" }

func main() {
	fmt.Print(1, 2, 3, "\n")
	fmt.Print("a", "b", "c", "\n")
	fmt.Print("a", 1, "b", 2, 3, "c", "\n")
	fmt.Print(1, "a", 2, "\n")
	fmt.Print(1.5, 2.5, true, false, 'x', "\n")
	fmt.Print(S("a"), S("b"), 1, S("c"), "\n")
	fmt.Print(E{}, E{}, St{}, St{}, "\n")
	fmt.Print(nil, nil, 1, nil, "x", nil, "\n")
	fmt.Print([]int{1}, []int{2}, "\n")
	fmt.Print("\n")
	fmt.Print()
	fmt.Println()
	fmt.Println(1, 2, 3)
	fmt.Println("a", "b")
	fmt.Println("a", 1, "b")
	fmt.Println(nil, nil)
	fmt.Println(S("x"), E{}, St{}, [2]St{}, "end")
	fmt.Printf("%q %q %q %q\n", fmt.Sprint(), fmt.Sprint(1, 2), fmt.Sprint("a", "b"), fmt.Sprint("a", 1, 2, "b"))
	fmt.Printf("%q %q %q\n", fmt.Sprintln(), fmt.Sprintln(1, "a", 2), fmt.Sprintln("a"))
	fmt.Printf("%q %q\n", fmt.Sprint(1, nil, 2), fmt.Sprint(nil, nil))
	fmt.Printf("%q %q\n", fmt.Sprint(St{}, 1, St{}), fmt.Sprint("x", St{}, "y"))
	var e error
	var i any
	fmt.Print(e, i, "|", e, i, "\n")
	fmt.Println(e, i)
}

// Output:
// 1 2 3
// abc
// a1b2 3c
// 1a2
// 1.5 2.5 true false 120
// ab1c
// err err st st
// <nil> <nil> 1 <nil>x<nil>
// [1] [2]
//
//
// 1 2 3
// a b
// a 1 b
// <nil> <nil>
// x err st [st st] end
// "" "1 2" "ab" "a1 2b"
// "\n" "1 a 2\n" "a\n"
// "1 <nil> 2" "<nil> <nil>"
// "st 1 st" "xsty"
// <nil> <nil>|<nil> <nil>
// <nil> <nil>
