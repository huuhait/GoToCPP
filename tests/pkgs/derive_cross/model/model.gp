// Package model has types with derived methods that other packages use.
package model

#Derive(String)
type Kind int

const (
	Book Kind = iota + 1
	Film
	Game
)

#Derive(Equal, Clone)
type Item struct {
	Kind  Kind
	Name  string
	Tags  []string
	Extra map[string]int
}

#Derive(Equal, Clone)
type Stack[T any] struct {
	Items []T
	Top   *T
}

// Push is a plain method next to the derived ones.
func (s *Stack[T]) Push(v T) {
	s.Items = append(s.Items, v)
	s.Top = &s.Items[len(s.Items)-1]
}

#Derive(Equal)
type secret struct {
	Code []int
}

func NewSecret(c ...int) Secret { return secret{c} }

// Secret is a type with an unexported implementation: callers use the interface.
type Secret interface{ Same(Secret) bool }

func (s secret) Same(o Secret) bool {
	t, ok := o.(secret)
	return ok && s.Equal(t)
}
