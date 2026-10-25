// Package example is a fake package for the tool's tests. Its declarations
// exercise every //go:linkname kind and a bodyless function.
package example

import _ "unsafe"

//go:linkname pullLocal example/dep.Pulled
func pullLocal()

//go:linkname pushLocal example/dep.Pushed
func pushLocal() {}

//go:linkname oneArg
func oneArg() {}

//go:linkname varLink example/dep.Var
var varLink int

//go:linkname linkerSym go:fipsinfo
var linkerSym uintptr

func body() {}

// bodyless is implemented in asm.s.
func bodyless() int
