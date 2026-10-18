// phase: 07
// ExitCode: 2
// Stderr: ^panic: myerr 7
// Stderr: (?m)^goroutine 1 \[running\]:

// Panics with values of the type parameter's type: the text Go prints for each kind of value.
package main

import (
	"errors"
	"fmt"
)

type MyErr struct{ code int }

func (e MyErr) Error() string { return fmt.Sprint("myerr ", e.code) }

type Str string

func (s Str) String() string { return "Str(" + string(s) + ")" }

func Recover[T any](v T) (r any) {
	defer func() { r = recover() }()
	panic(v)
}

func Throw[T any](v T) { panic(v) }

func main() {
	fmt.Println(Recover(1), Recover("s"), Recover(2.5), Recover(true), Recover(Str("x")))
	fmt.Println(Recover(MyErr{3}), Recover[error](errors.New("plain")), Recover([]int{1}), Recover(struct{ A int }{4}))
	Throw(MyErr{7})
}

// Output:
// 1 s 2.5 true Str(x)
// myerr 3 plain [1] {4}
