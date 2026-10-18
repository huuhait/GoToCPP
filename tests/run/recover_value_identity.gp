// phase: 05

package main

import "fmt"

type MyErr struct{ Code int }

func (e *MyErr) Error() string { return fmt.Sprint("myerr ", e.Code) }

// The value that recover returns is the value that was panicked: the same pointer.
func main() {
	want := &MyErr{9}
	var got any
	func() {
		defer func() { got = recover() }()
		panic(want)
	}()
	fmt.Println(got == any(want))
	var err error = want
	var got2 any
	func() {
		defer func() { got2 = recover() }()
		panic(err)
	}()
	fmt.Println(got2 == any(err), got2 == got)
	p, ok := got.(*MyErr)
	fmt.Println(ok, p == want, p.Code)
}

// Output:
// true
// true true
// true true 9
