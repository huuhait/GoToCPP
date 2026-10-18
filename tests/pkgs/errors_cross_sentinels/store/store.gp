package store

import "errors"

var (
	ErrNotFound = errors.New("not found")
	ErrClosed   = errors.New("closed")
)

type Store struct {
	data   map[string]int
	closed bool
}

func New() *Store { return &Store{data: map[string]int{"a": 1}} }

func (s *Store) Close() { s.closed = true }

func (s *Store) Get(k string) (int, error) {
	if s.closed {
		return 0, ErrClosed
	}
	v, ok := s.data[k]
	if !ok {
		return 0, ErrNotFound
	}
	return v, nil
}
