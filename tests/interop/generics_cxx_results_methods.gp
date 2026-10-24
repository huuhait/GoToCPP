// phase: 08

// The result of a deduced generic call in a method of a generic type and in a generic function: the
// instance of the type (and its methods) is lowered after every package (lower: finishShared), with the
// types that the lowering gave the result (lower/deducedres.go) still in the package's maps.
package main

import (
	"fmt"
	"slices"

	"<string>"
)

type Box[T any] struct{ V T }

func (b Box[T]) N(c []std.string) int {
	r := slices.Insert(c, 0, std.string("x"))
	return len(r) + cap(r[:1])
}

func (b *Box[T]) M(c []std.string) (T, int) {
	var r = slices.Insert(c, 1, std.string("y"))
	return b.V, len(r[1:])
}

func G[T any](v T, c []std.string) int {
	r := slices.Insert(c, 0, std.string("g"))
	return len(r)
}

func main() {
	words := []std.string{"a", "b"}
	fmt.Println(Box[int]{V: 1}.N(words), Box[string]{}.N(words), G(1, words), G("s", words))
	b := &Box[string]{V: "v"}
	fmt.Println(b.M(words))
}

// Output:
// 7 7 3 3
// v 2
