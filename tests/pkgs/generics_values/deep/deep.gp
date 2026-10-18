package deep

import "fmt"

var Trace []string

func Log[T any](v T) T {
	Trace = append(Trace, fmt.Sprint(v))
	return v
}

func Twice[T any](f func(T) T) func(T) T { return func(x T) T { return f(f(x)) } }
