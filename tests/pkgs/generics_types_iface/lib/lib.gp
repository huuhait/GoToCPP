package lib

type Getter[T any] interface{ Get() T }
type Setter[T any] interface{ Set(T) }

type Cell[T any] interface {
	Getter[T]
	Setter[T]
}

type Box[T any] struct{ v T }

func (b *Box[T]) Get() T  { return b.v }
func (b *Box[T]) Set(v T) { b.v = v }

func NewBox[T any](v T) Cell[T] { return &Box[T]{v} }

func Swap[T any](a, b Cell[T]) {
	x, y := a.Get(), b.Get()
	a.Set(y)
	b.Set(x)
}

func Read[T any](g Getter[T]) T { return g.Get() }
