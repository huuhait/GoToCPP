// phase: 09

// Package util declares the macros that package main uses (the C++ of the use must hold their code).
package util

import (
	"errors"
	"fmt"
)

// Count is how often a tallied function ran.
var Count int

// ErrNegative is returned by Step for a negative accumulator.
var ErrNegative = errors.New("negative")

// Assert panics with the place and the text of the condition when cond is false; msg is lazy.
macro Assert(cond bool, msg lazy string) {
	if !cond {
		panic(fmt.Sprintf("%s:%d: %s: %s", #file(), #line(), #text(cond), msg()))
	}
}

// Trace prints when the function starts and when it ends.
macro Trace() func() {
	fmt.Println("enter", #func())
	return func() { fmt.Println("exit ", #func()) }
}

// Tally counts the calls of the function it is written above, then runs the function's code.
macro Tally() {
	Count++
	#body()
}

// Dbg returns v after printing it with its place.
macro Dbg[T any](v T) T {
	fmt.Printf("[%s:%d] %s = %v\n", #file(), #line(), #text(v), v)
	return v
}

// Step returns a new accumulator, or an error for a negative one.
func Step(i, acc int) (int, error) {
	if acc < 0 {
		return 0, ErrNegative
	}
	return (acc*31 + i) & 0xffff, nil
}

// Load is a must function: the dropped error is returned.
func Load!(n int) (int, error) {
	v := Step(n, 1)
	return v * 2, nil
}
