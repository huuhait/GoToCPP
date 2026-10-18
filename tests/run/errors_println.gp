// phase: 05
// Stderr: (?m)^plain$
// Stderr: (?m)^a\nb$

// The builtin println of an error's text goes to standard error, with no formatting package.

package main

import "errors"

func main() {
	err := errors.New("plain")
	println(err.Error())
	println(errors.Join(errors.New("a"), errors.New("b")).Error())
}

// Output:
