// Package lib has a generic type whose method, and a plain function, use the result of a deduced
// generic call.
package lib

import (
	"slices"

	"<string>"
)

type Box[T any] struct{ V T }

func (b Box[T]) N(c []std.string) int {
	r := slices.Insert(c, 0, std.string("x"))
	return len(r) + cap(r[:1])
}

func Plain(c []std.string) int {
	r := slices.Insert(c, 0, std.string("x"))
	return len(r)
}
