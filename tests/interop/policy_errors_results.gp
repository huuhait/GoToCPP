// phase: 04

// The policy table of docs/spec/interop.md ("Types that can't be copied"), results: a named
// result has a zero value from the start; a function with defers returns a result from a
// variable, so its type must be movable; a local that is returned is moved.
package main

import (
	"./objects.h"
)

func newPinned() objects.Pinned { return objects.Pinned(1) }

// A named result has a zero value from the start.
func namedResult() (w objects.Widget) { // ERROR "C\+\+: .*(no matching|constructor)"
	return objects.Widget(1)
}

// A function with defers returns a result from a variable: a type with no move is no result.
func pinnedResult() objects.Pinned {
	defer func() {}()
	return newPinned() // ERROR "C\+\+: the C\+\+ type of this result can't be moved or copied"
}

// A local of a type with no move can't be returned (C++ moves a local that is returned).
func pinnedLocal() objects.Pinned {
	p := newPinned()
	return p // ERROR "C\+\+: .*(deleted|move)"
}

func main() {
	namedResult()
	pinnedResult()
	pinnedLocal()
}
