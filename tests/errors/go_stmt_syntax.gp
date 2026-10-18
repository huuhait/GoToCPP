// phase: 08

// A go statement needs a call (a syntax error): not a function value and not a func literal that is
// not called. The type errors of go statements are in chan_misuse.gp.
package main

func f() int { return 1 }

func main() {
	go f           // ERROR "expression in go must be function call"
	go func() {}   // ERROR "expression in go must be function call"
}
