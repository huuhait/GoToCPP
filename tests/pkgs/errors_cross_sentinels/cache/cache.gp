package cache

import (
	"errors"

	"example.com/sent/store"
)

// ErrNotFound has the text of store's, and is a different error.
var ErrNotFound = errors.New("not found")

type missErr struct {
	key string
	err error
}

func (m *missErr) Error() string { return "cache miss for " + m.key + ": " + m.err.Error() }
func (m *missErr) Unwrap() error { return m.err }

func Lookup(s *store.Store, k string) (int, error) {
	v, err := s.Get(k)
	if err != nil {
		return 0, &missErr{k, err}
	}
	return v, nil
}

func Own(k string) error {
	if k == "" {
		return ErrNotFound
	}
	return nil
}
