package a

import "example.com/anonfwd/b"

// The same unnamed struct type (embedding a type of package b) is used in packages a, c and main:
// it is defined once for the program, its forwarders by the first package that has it (the owner).
func Make(n int) struct{ b.T } { return struct{ b.T }{b.T{N: n}} }

func Show(v struct{ b.T }) int { return v.Get() }
