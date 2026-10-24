// phase: 07

// The result of a generic call whose type arguments C++ deduces is a gocpp slice or map of C/C++
// values when the generic result type is one (S ~[]E, []E, map[K]V): it has no members, directly or
// through a variable declared from it, as the build says (lower/deducedres.go); check says so too.
// With every type argument written, or a result that is a T, the value is a C/C++ value.
package main

import (
	"fmt"
	"maps"
	"slices"

	"<string>"
)

func Grow[T any, S ~[]T](xs S, x T) S { return append(xs[:len(xs):len(xs)], x) }

func First[T any](xs []T, x T) T { return x }

func main() {
	c := []std.string{"a"}
	r := Grow(c, std.string("q"))
	fmt.Println(int(r.size())) // ERROR "r.size undefined \(r is a gocpp slice of C/C\+\+ values, which has no field or method size\)"
	var r2 = slices.Insert(c, 0, std.string("z"))
	fmt.Println(r2.empty())                                   // ERROR "r2.empty undefined \(r2 is a gocpp slice of C/C\+\+ values, which has no field or method empty\)"
	fmt.Println(slices.Insert(c, 0, std.string("y")).front()) // ERROR "front undefined \(slices.Insert\(c, 0, std.string\(.y.\)\) is a gocpp slice of C/C\+\+ values"
	m := maps.Clone(map[int]std.string{1: std.string("v")})
	fmt.Println(m.size()) // ERROR "m.size undefined \(m is a gocpp map of C/C\+\+ values, which has no field or method size\)"
	fmt.Println(int(First(c, std.string("w")).size()))
}
