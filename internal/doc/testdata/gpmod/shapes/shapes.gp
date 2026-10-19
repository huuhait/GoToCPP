// Package shapes shows generic types and functions, embedded fields, and
// interfaces.
package shapes

import "example.com/gp/macros"

// Stack is a stack of T.
type Stack[T any] struct {
	items []T
}

// Push adds v on top.
func (s *Stack[T]) Push(v T) { s.items = append(s.items, v) }

// Pop removes the top.
func (s *Stack[T]) Pop() (v T, ok bool) { return }

// NewStack returns an empty stack.
func NewStack[T any]() *Stack[T] { return &Stack[T]{} }

// Pair holds two values.
type Pair[K comparable, V any] struct {
	Key K
	Val V
}

// Map applies f to each element of xs.
func Map[T, U any](xs []T, f func(T) U) []U { return nil }

// Number is a constraint.
type Number interface {
	~int | ~int64 | ~float64
}

// Sum adds the numbers.
func Sum[N Number](xs ...N) N { var z N; return z }

// Shape is something with an area.
type Shape interface {
	// Area is the area.
	Area() float64
	// Name is the name.
	Name() string
}

// Base is the part that every shape has.
type Base struct {
	// ID identifies the shape.
	ID int
	tag string
}

// Circle is a Shape.
type Circle struct {
	Base
	*macros.Point
	Radius float64 // in meters
}

// Area implements Shape.
func (c Circle) Area() float64 { return 3 * c.Radius * c.Radius }

// Name implements Shape.
func (c Circle) Name() string { return "circle" }

// Deprecated: use Circle.
type Disc = Circle

// Old does nothing.
//
// Deprecated: use New.
func Old() {}

// Level is a level.
type Level uint8

const (
	// Low is the lowest.
	Low Level = iota + 1
	// Mid is in the middle.
	Mid
	// High is the highest.
	High
)

const secret = 1

// Default is the default level.
var Default = Mid

// MaxLevel is the maximum.
const MaxLevel Level = 9

func helper() {}
