package lib

import "example.com/gvalues/deep"

// Traced calls deep's generic functions from a generic body, so main need not import deep.
func Traced[T any](f func(T) T) func(T) T {
	return deep.Twice(func(x T) T { return deep.Log(f(x)) })
}

func Inc[T ~int | ~float64](x T) T { return x + 1 }

func Compose[A, B, R any](f func(A) B, g func(B) R) func(A) R { return func(a A) R { return g(f(a)) } }

func Trace() []string { return deep.Trace }
