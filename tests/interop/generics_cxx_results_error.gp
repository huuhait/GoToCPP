// phase: 07

// The result of a generic call whose type arguments C++ deduces is a gocpp slice of C++ values, not a
// C++ container: it has no members, as Go's slice has none.
package main

import (
	"fmt"

	"<string>"
)

func Grow[T any, S ~[]T](xs S, x T) S { return append(xs[:len(xs):len(xs)], x) }

func main() {
	c := []std.string{"a", "b"}
	r := Grow(c, std.string("q"))
	fmt.Println(int(r.size())) // ERROR "r.size undefined \(r is a gocpp slice of C/C\+\+ values, which has no field or method size\)"
}
