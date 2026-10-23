// phase: 07

// A package variable without a type whose value is a call of a generic function whose type argument C++ deduces has no type that
// gocpp can write: check, vet and the build say so at the variable, and name the type arguments to write; written type arguments
// (var ok = Seq[std.string](words)) or a written type work.
package main

import (
	"fmt"

	"<string>"
)

func Seq[T any](xs []T) func(yield func(T) bool) {
	return func(yield func(T) bool) {
		for _, x := range xs {
			if !yield(x) {
				return
			}
		}
	}
}

func Keys[M ~map[K]V, K comparable, V any](m M) M { return m }

var words = []std.string{"a", "bcd"}
var it = Seq(words)                // ERROR "package variables of C/C\+\+ values without a type are not supported yet: write the type of it \(var it T = ...\) or the type arguments of Seq \(Seq\[std.string\]\(...\)\)"
var k = Keys(map[int]std.string{}) // ERROR "write the type of k \(var k T = ...\) or the type arguments of Keys \(Keys\[map\[int\]std.string\]\(...\)\)"
var ok = Seq[std.string](words)
var typed func(func(std.string) bool) = Seq(words)

func main() {
	_, _, _, _ = it, k, ok, typed
	fmt.Println()
}
