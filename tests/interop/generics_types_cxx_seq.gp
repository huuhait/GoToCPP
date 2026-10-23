// phase: 07

// An iter.Seq with a C++ type argument is an instance of a generic type with a C/C++ type argument, written out (iter.Seq[std.string],
// in a package variable and a local one) or made by a generic function whose type argument C++ deduces (SeqOf(words) returns
// iter.Seq[T], slices.Values too): check, vet and the build give the same errors, at the variable's type and at the use of the function.
package main

import (
	"fmt"
	"iter"
	"slices"

	"<string>"
)

func SeqOf[T any](xs []T) iter.Seq[T] {
	return slices.Values(xs)
}

var all iter.Seq[std.string] // ERROR "an instance of a generic type with a C/C\+\+ type argument \(iter.Seq\[C.std.string\]\) is not supported yet: the type arguments of a generic type are Go types"

func main() {
	words := []std.string{"a", "bcd"}
	var s iter.Seq[std.string] // ERROR "an instance of a generic type with a C/C\+\+ type argument \(iter.Seq\[C.std.string\]\) is not supported yet"
	_, _ = s, all
	for w := range SeqOf(words) { // ERROR "an instance of the generic type Seq with a C/C\+\+ type argument of a generic function is not supported yet \(SeqOf makes iter.Seq\[T\]\): the type arguments of a generic type are Go types"
		fmt.Println(int(w.size()))
	}
	for w := range slices.Values(words) { // ERROR "an instance of the generic type Seq with a C/C\+\+ type argument of a generic function is not supported yet \(Values makes iter.Seq\[E\]\)"
		fmt.Println(int(w.size()))
	}
	for w := range SeqOf([]int{1}) {
		fmt.Println(w)
	}
}
