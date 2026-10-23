// phase: 07

// What gocpp cannot build with C/C++ type arguments of generic code, which check reports as the build does: an instance of a
// generic type with a C/C++ type argument wherever it is written (a package variable, a local, a field, a parameter, a result,
// a composite literal, a nested instance once, an alias of an instance, a type declared in a function, iter.Seq), a generic
// function whose instance makes one (at its use), and a package variable whose C/C++ value has no type gocpp can write. A Go type
// argument, an alias of an unnamed type, written type arguments of a function, and a written variable type check clean.
package main

import (
	"fmt"
	"iter"

	"<string>"
)

type Box[T any] struct{ v T }

type Vec[T any] = []T

type Al[T any] = Box[T]

type G struct{ s std.string }

var pkgBox Box[std.string] // ERROR "an instance of a generic type with a C/C\+\+ type argument \(Box\[C.std.string\]\) is not supported yet: the type arguments of a generic type are Go types"

type S struct {
	b Box[std.string] // ERROR "\(Box\[C.std.string\]\) is not supported yet"
}

func param(b Box[std.string]) {} // ERROR "\(Box\[C.std.string\]\) is not supported yet"

func result() (b Al[std.string]) { return } // ERROR "\(Al\[C.std.string\]\) is not supported yet"

func Wrap[T any](v T) int {
	b := Box[T]{v}
	_ = b
	return 1
}

func Seq[T any](xs []T) func(yield func(T) bool) {
	return func(yield func(T) bool) {}
}

func Id[T any](x T) T { return x }

var words = []std.string{"a"}
var it = Seq(words) // ERROR "package variables of C/C\+\+ values without a type are not supported yet: write the type of it \(var it T = ...\) or the type arguments of Seq \(Seq\[std.string\]\(...\)\)"
var ok = Seq[std.string](words)
var typed func(func(std.string) bool) = Seq(words)

func main() {
	var b Box[std.string]      // ERROR "\(Box\[C.std.string\]\) is not supported yet"
	_ = Box[std.string]{}      // ERROR "\(Box\[C.std.string\]\) is not supported yet"
	var bb []Box[Box[G]]       // ERROR "\(Box\[Box\[G\]\]\) is not supported yet"
	var s iter.Seq[std.string] // ERROR "\(iter.Seq\[C.std.string\]\) is not supported yet"
	type L[T any] struct{ v T }
	var l L[std.string]                // ERROR "\(L\[C.std.string\]\) is not supported yet"
	fmt.Println(Wrap(std.string("x"))) // ERROR "an instance of the generic type Box with a C/C\+\+ type argument of a generic function is not supported yet \(Wrap makes Box\[T\]\): the type arguments of a generic type are Go types"
	var v Vec[std.string]
	var gb Box[int]
	_, _, _, _, _, _, _, _, _, _, _ = b, bb, s, l, v, gb, pkgBox, it, ok, typed, Id[std.string](words[0])
	fmt.Println(Wrap(1), S{}, param, result)
}
