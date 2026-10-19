package box

type Box[T any] struct{ V T }

func (b Box[T]) Get() T { return b.V }

type Pair[A, B any] struct {
	First  A
	Second B
}

func Wrap[T any](v T) Box[Box[T]] { return Box[Box[T]]{Box[T]{v}} }
