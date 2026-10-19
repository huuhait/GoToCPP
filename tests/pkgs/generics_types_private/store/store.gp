// Package store: an exported generic type whose methods use unexported names, types and generic functions of the package.
package store

import "fmt"

type entry[V any] struct {
	key string
	val V
	n   int
}

var counter int

const limit = 3

func tick() int { counter++; return counter }

func label[T any](v T) string { return fmt.Sprintf("<%v>", v) }

type Store[V any] struct {
	entries []entry[V]
	hidden  map[string]int
}

func New[V any]() *Store[V] { return &Store[V]{hidden: map[string]int{}} }

func (s *Store[V]) Put(key string, v V) string {
	if len(s.entries) >= limit {
		panic("full: " + key)
	}
	s.entries = append(s.entries, entry[V]{key, v, tick()})
	s.hidden[key]++
	return label(v) + label(key)
}

func (s *Store[V]) Last() (V, int) {
	e := s.entries[len(s.entries)-1]
	return e.val, e.n
}

func (s Store[V]) Keys() []string {
	var ks []string
	for _, e := range s.entries {
		ks = append(ks, e.key)
	}
	return ks
}

func Counter() int { return counter }

func Raw[V any](s *Store[V]) any { return s.entries }
