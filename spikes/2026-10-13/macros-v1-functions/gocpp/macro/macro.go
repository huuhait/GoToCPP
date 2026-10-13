// Package macro: what macro definitions see (a tiny real package).
package macro

import "fmt"

// Site is filled in by the compiler at each #call (a static constant: zero runtime cost).
type Site struct {
	File string
	Line int
	Func string
	Args []string // source text of each argument as written at the call site
}

func (s *Site) Pos() string       { return fmt.Sprintf("%s:%d", s.File, s.Line) }
func (s *Site) Text(i int) string { return s.Args[i] }

// Lazy[T] is an argument that is only evaluated when Get is called.
type Lazy[T any] func() T

func (l Lazy[T]) Get() T { return l() }
