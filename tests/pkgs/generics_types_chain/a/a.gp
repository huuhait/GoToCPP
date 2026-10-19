package a

type Box[T any] struct{ V T }

func (b Box[T]) Get() T      { return b.V }
func (b *Box[T]) Set(v T)    { b.V = v }

type Meters float64

func Wrap[T any](v T) Box[T] { return Box[T]{v} }
