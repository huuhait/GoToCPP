package lib

import "example.com/borrow/inner"

// Wrapper embeds a type of another package, by value and through a pointer.
type Wrapper struct {
	inner.Counter
	Name string
}

type PtrWrapper struct {
	*inner.Counter
	Extra int
}

type Incer interface {
	Inc() int
	Add(int)
	Get() int
}

// Bump calls the borrowed methods through promotion and an interface.
func Bump(w *Wrapper, k int) int {
	w.Add(k)
	w.Inc()
	return w.Get() + inner.Peek(&w.Counter)
}

func ViaIface(i Incer, k int) int {
	i.Add(k)
	return i.Inc()
}

// Each calls f on the address of every element: a func value with a pointer parameter.
func Each(ws []Wrapper, f func(*inner.Counter)) {
	for i := range ws {
		f(&ws[i].Counter)
	}
}

// Stack is generic: its methods are not analysed (they keep owning receivers), and calls into
// borrowing functions of another package from an instance go through the same lending.
type Stack[T any] struct{ items []T }

func (s *Stack[T]) Push(v T) { s.items = append(s.items, v) }
func (s *Stack[T]) Len() int  { return len(s.items) }

func Fill[T any](s *Stack[T], vs ...T) int {
	for _, v := range vs {
		s.Push(v)
	}
	return s.Len()
}

func PeekAll(cs []*inner.Counter) (n int) {
	for _, c := range cs {
		n += inner.Peek(c)
	}
	return
}

func Both(c *inner.Counter) int { return inner.Peek(c) + c.Get() }
