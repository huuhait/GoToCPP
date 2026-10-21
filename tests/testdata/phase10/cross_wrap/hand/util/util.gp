// phase: 09

// Package util has a wrap macro and a function with an error result, used from package main.
package util

import "errors"

// Count is how often a tallied function ran.
var Count int

var errNegative = errors.New("negative")

// Tally counts the calls of the function it is written above, then runs the function's code.
macro Tally() {
	Count++
	#body()
}

// Step returns a new accumulator, or an error for a negative one.
func Step(i, acc int) (int, error) {
	if acc < 0 {
		return 0, errNegative
	}
	return (acc*31 + i) & 0xffffff, nil
}
