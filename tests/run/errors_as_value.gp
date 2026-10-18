// phase: 05

// errors.As into a struct VALUE target (the error type has a value receiver): the value is copied.

package main

import (
	"errors"
	"fmt"
)

type ValErr struct {
	Code int
	Tags [2]string
}

func (e ValErr) Error() string { return fmt.Sprintf("valerr %d %s", e.Code, e.Tags[0]) }

type Celsius float64

func (c Celsius) Error() string { return "too hot" }

type wrap struct{ err error }

func (w wrap) Error() string { return "wrap" }
func (w wrap) Unwrap() error { return w.err }

func main() {
	src := ValErr{7, [2]string{"x", "y"}}
	err := wrap{src}
	var v ValErr
	fmt.Println(errors.As(err, &v), v.Code, v.Tags[1], v == src)
	v.Code = 8
	fmt.Println(src.Code, v.Error())
	var c Celsius
	fmt.Println(errors.As(wrap{Celsius(41.5)}, &c), float64(c))
	fmt.Println(errors.As(wrap{Celsius(1)}, &v), v.Code)
	var w wrap
	fmt.Println(errors.As(wrap{wrap{src}}, &w), w.err == error(src))
}

// Output:
// true 7 y true
// 7 valerr 8 x
// true 41.5
// false 8
// true false
