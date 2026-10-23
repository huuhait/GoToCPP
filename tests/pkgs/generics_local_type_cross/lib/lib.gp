package lib

import "fmt"

// Make returns a value of a generic type declared in a generic function of this package.
func Make[T any](v T) any {
	type Box[U any] struct {
		u U
		t T
	}
	return Box[string]{"lib", v}
}

// Plain returns a value of a generic type declared in a function that is not generic.
func Plain() any {
	type Box[U any] struct{ u U }
	return Box[int]{1}
}

type Stack[T any] struct{ items []T }

func NewStack[T any](items ...T) Stack[T] { return Stack[T]{items} }

// Cell returns a value of a generic type declared in a method of a generic type.
func (s Stack[T]) Cell() any {
	type Cell[U any] struct {
		u U
		t T
	}
	return Cell[bool]{true, s.items[0]}
}

func Describe(v any) string { return fmt.Sprintf("%T %v", v, v) }
