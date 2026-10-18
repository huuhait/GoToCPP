// phase: 07

// A generic function whose type argument C++ deduces cannot be a func value: the message asks for the type arguments.
package main

import "<vector>"

func Id[T any](x T) T { return x }

func main() {
	var h func(std.vector[int]) std.vector[int] = Id // ERROR "a generic function with a type argument that C\+\+ deduces cannot be a func value: write the type arguments"
	_ = h
}
