// Package util is the module's util package with its macros written out the way the compiler does:
// each macro is a function whose first parameters are the place of the outermost use (file, line,
// function) and the text of the arguments, a lazy parameter is a func value, and the code of a wrap
// macro is a func value.
package util

import "fmt"

type Level int

type Result struct {
	Name string
	OK   bool
}

var calls int

func Calls() int { return calls }

func note(kind, where string) {
	calls++
	fmt.Println(kind, where)
}

func Assert(file string, line int, fn string, text string, cond bool, msg func() string) {
	if !cond {
		note("assert", fmt.Sprintf("%s:%d in %s: %s: %s", file, line, fn, text, msg()))
	}
}

func Trace(fn string) func() {
	note("enter", fn)
	return func() { note("exit", fn) }
}

func VM(fn string, body func()) {
	note("vm", fn)
	body()
}

func Describe(r func() Result, l func() Level) string {
	return fmt.Sprint(r().Name, ":", r().OK, ":", l())
}

func Dbg[T any](text string, v T) T {
	note("dbg", fmt.Sprintf("%s = %v", text, v))
	return v
}

func local(n int) int { return n + 1 }

func Internal(n int) int {
	Assert("util/util.gp", 58, "example.com/macros/util.Internal", "n > 0", n > 0, func() string { return "n is positive" })
	return local(Dbg("n", n))
}
