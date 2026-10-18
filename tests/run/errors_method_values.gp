// phase: 05

// Method values and expressions on error: bound to the value when taken.

package main

import (
	"errors"
	"fmt"
)

type counter struct{ n int }

func (c *counter) Error() string {
	c.n++
	return fmt.Sprint("called ", c.n)
}

func main() {
	var err error = &counter{}
	f := err.Error
	fmt.Println(f(), f())
	g := error.Error
	fmt.Println(g(err))
	err = errors.New("replaced")
	fmt.Println(f(), err.Error())
	h := errors.Unwrap
	fmt.Println(h(err) == nil)
	var fs []func(error, error) bool
	fs = append(fs, errors.Is)
	fmt.Println(fs[0](err, err), fs[0](err, nil))
}

// Output:
// called 1 called 2
// called 3
// called 4 replaced
// true
// true false
