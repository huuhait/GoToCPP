// Package lib has an API with C++ values: functions, methods, func fields, package variables, a
// generic struct with a func field and an iterator variable.
package lib

import "<string>"

type NH struct {
	F func(std.string) int
}

func (n NH) Call(s std.string) int { return int(s.size()) }

func Len(s std.string) int { return int(s.size()) }

type H[T any] struct {
	F func(std.string) T
	N T
}

func (h H[T]) Call(s std.string) T { return h.F(s) }

var Words = []std.string{"a", "b"}

var Ws []std.string = Words

var It std.string = std.string("xyz")

func Seq[T any](xs []T) func(yield func(T) bool) {
	return func(yield func(T) bool) {
		for _, x := range xs {
			if !yield(x) {
				return
			}
		}
	}
}

var Iter = Seq[std.string](Words)

var IterT func(yield func(std.string) bool) = Seq[std.string](Words)
