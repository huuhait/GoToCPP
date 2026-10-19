// phase: 10

// Package util declares macros for the packages that import it. Their bodies use util's private
// names (note, calls), which the C++ code of a use in another package names with util's namespace.
package util

import "fmt"

type Level int

type Result struct {
	Name string
	OK   bool
}

var calls int

// Calls returns how often a macro of this package reported.
func Calls() int { return calls }

func note(kind, where string) {
	calls++
	fmt.Println(kind, where)
}

// Assert reports a false condition with the place of the outermost use.
macro Assert(cond bool, msg lazy string) {
	if !cond {
		note("assert", fmt.Sprintf("%s:%d in %s: %s: %s", #file(), #line(), #func(), #text(cond), msg()))
	}
}

macro Trace() func() {
	note("enter", #func())
	return func() { note("exit", #func()) }
}

macro VM() {
	note("vm", #func())
	#body()
}

// Describe uses lazy parameters of types of this package: the use writes util.Result.
macro Describe(r lazy Result, l lazy Level) string {
	return fmt.Sprint(r().Name, ":", r().OK, ":", l())
}

macro Dbg[T any](v T) T {
	note("dbg", fmt.Sprintf("%s = %v", #text(v), v))
	return v
}

// local is a macro that only this package can use.
macro local(n int) int { return n + 1 }

// Internal uses macros of its own package, also the unexported one.
func Internal(n int) int {
	#Assert(n > 0, "n is positive")
	return #local(#Dbg(n))
}
