// phase: 10

// What the compiler says about a `!` call that cannot work (step 10.3): the last result of the
// callee must be exactly error (a *MyErr would turn a nil pointer into a non-nil error), the
// number of values must fit, and `go` and `defer` have no place to return the error to.
package main

import (
	"fmt"
	"os"
)

type MyErr struct{}

func (*MyErr) Error() string { return "my" }

func typed() (int, *MyErr)               { return 0, nil }
func onlyInt() int                       { return 1 }
func pairBool() (int, bool)              { return 1, true }
func errOnly() error                     { return nil }
func two() (int, error)                  { return 1, nil }
func three() (int, int, int, int, error) { return 1, 2, 3, 4, nil }
func none()                              {}

type S struct{}

func (S) M() (int, error) { return 1, nil }

func main() {
	a := typed!()          // ERROR "typed! requires the last result of typed to be exactly error, got \*MyErr: a nil \*MyErr would turn into a non-nil error; convert it explicitly"
	b := onlyInt!()        // ERROR "onlyInt! requires the last result of onlyInt to be error, got int"
	c := pairBool!()       // ERROR "pairBool! requires the last result of pairBool to be error, got bool"
	d := errOnly!()        // ERROR "errOnly!\(...\) returns only an error: use it as a statement"
	e, f := two!()         // ERROR "two!\(...\) gives 1 value besides the error, but here 2 values are wanted"
	var g int = two!()
	x := 5
	h := x!()              // ERROR "cannot call x \(variable of type int\)"
	go two!()              // ERROR "go two!\(...\): the error of two can't be returned to anyone from another goroutine"
	i := three!()          // ERROR "three!\(...\) gives 4 values besides the error: assign them to 4 variables"
	j := undefined!(1)     // ERROR "undefined: undefined"
	k := os.Remove!("x") + 1 // ERROR "os.Remove!\(...\) returns only an error: use it as a statement"
	var s S
	_ = s.M!()
	l := fmt.Sprint!(1)    // ERROR "fmt.Sprint! requires the last result of fmt.Sprint to be error, got string"
	none!()                // ERROR "none! requires the last result of none to be error, but none returns no result"
	m := len!("abc")       // ERROR "len!\(...\) is a call of a built-in function"
	n := int!(5)           // ERROR "int!\(...\) is a conversion, not a call"
	fmt.Println(a, b, c, d, e, f, g, h, i, j, k, l, m, n)
}
