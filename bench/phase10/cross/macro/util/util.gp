// phase: 09

// Package util has the macros of the cross-package benchmark.
package util

import "fmt"

var Count int

macro Assert(cond bool, msg lazy string) {
	if !cond {
		panic(fmt.Sprintf("%s:%d: %s: %s", #file(), #line(), #text(cond), msg()))
	}
}

macro Tally() {
	Count++
	#body()
}
