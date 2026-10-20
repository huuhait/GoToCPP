package lib

var calls int

func identity[T any](v T) T { return v }

type Box[T any] struct {
	Items []T
}

func (b *Box[T]) Add(v T) {
	b.Items = append(b.Items, v)
	calls++
}

func (b Box[T]) First() T { return identity(b.Items[0]) }

func (b Box[T]) Last() T {
	var last T
	for _, v := range b.Items {
		last = identity(v)
	}
	return last
}

func (b Box[T]) Reader() func() T {
	return func() T {
		return b.Last()
	}
}

type Wrapped[T any] struct {
	Box[T]
}

func Calls() int { return calls }
