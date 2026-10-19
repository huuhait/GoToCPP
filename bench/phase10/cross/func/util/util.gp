// phase: 09

// Package util has the functions that a Go programmer without macros would write (variant func): the check takes
// the message as a func value and the wrap takes the code as a func value.
package util

import "fmt"

var Count int

func Assert(file string, line int, cond bool, text string, msg func() string) {
	if !cond {
		panic(fmt.Sprintf("%s:%d: %s: %s", file, line, text, msg()))
	}
}

func Tally(body func()) {
	Count++
	body()
}
