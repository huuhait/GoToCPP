// phase: 09

// Package lib has a package-level variable whose initializer has a must call: with no error result to
// return it to, a failure is a panic with the error (Go's Must convention).
package lib

import (
	"fmt"
	"strconv"
)

var Port = strconv.Atoi!(portText)

var portText = text()

func text() string {
	fmt.Println("lib: text")
	return "80x"
}

func init() {
	fmt.Println("lib: init (not reached)")
}
