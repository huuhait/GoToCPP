// Package model has types with methods that other packages use (written by hand).
package model

import "strconv"

type Kind int

const (
	Book Kind = iota + 1
	Film
	Game
)

func (k Kind) String() string {
	switch k {
	case Book:
		return "Book"
	case Film:
		return "Film"
	case Game:
		return "Game"
	}
	return "Kind(" + strconv.Itoa(int(k)) + ")"
}

type Item struct {
	Kind  Kind
	Name  string
	Tags  []string
	Extra map[string]int
}

func (i Item) Equal(o Item) bool {
	if i.Kind != o.Kind || i.Name != o.Name {
		return false
	}
	if len(i.Tags) != len(o.Tags) {
		return false
	}
	for n := range i.Tags {
		if i.Tags[n] != o.Tags[n] {
			return false
		}
	}
	if len(i.Extra) != len(o.Extra) {
		return false
	}
	for k, v := range i.Extra {
		ov, ok := o.Extra[k]
		if !ok || v != ov {
			return false
		}
	}
	return true
}

func (i Item) Clone() Item {
	out := i
	if i.Tags != nil {
		out.Tags = make([]string, len(i.Tags))
		copy(out.Tags, i.Tags)
	}
	if i.Extra != nil {
		out.Extra = make(map[string]int, len(i.Extra))
		for k, v := range i.Extra {
			out.Extra[k] = v
		}
	}
	return out
}

type Stack[T any] struct {
	Items []T
	Top   *T
}

func (s Stack[T]) Equal(o Stack[T]) bool {
	if len(s.Items) != len(o.Items) {
		return false
	}
	for i := range s.Items {
		if any(s.Items[i]) != any(o.Items[i]) {
			return false
		}
	}
	if s.Top != o.Top {
		if s.Top == nil || o.Top == nil || any(*s.Top) != any(*o.Top) {
			return false
		}
	}
	return true
}

func (s Stack[T]) Clone() Stack[T] {
	out := s
	if s.Items != nil {
		out.Items = make([]T, len(s.Items))
		copy(out.Items, s.Items)
	}
	if s.Top != nil {
		c := *s.Top
		out.Top = &c
	}
	return out
}

func (s *Stack[T]) Push(v T) {
	s.Items = append(s.Items, v)
	s.Top = &s.Items[len(s.Items)-1]
}

type secret struct {
	Code []int
}

func (s secret) Equal(o secret) bool {
	if len(s.Code) != len(o.Code) {
		return false
	}
	for i := range s.Code {
		if s.Code[i] != o.Code[i] {
			return false
		}
	}
	return true
}

func NewSecret(c ...int) Secret { return secret{c} }

type Secret interface{ Same(Secret) bool }

func (s secret) Same(o Secret) bool {
	t, ok := o.(secret)
	return ok && s.Equal(t)
}
