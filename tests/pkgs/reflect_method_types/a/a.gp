package a

import "fmt"

type T struct{ N int }
type U []T

func (t T) Show(u U) string               { return fmt.Sprint(t.N, len(u)) }
func (t *T) Use(i interface{ Get() T }) U { return nil }
func (t T) Keyed(m map[T]*U) [2]U         { return [2]U{} }

type Box[E any] struct{ v E }

func (b Box[E]) Get() E                    { return b.v }
func (b *Box[E]) Each(f func(int, E) bool) {}
func (b Box[E]) Pair() (Box[E], *Box[E])   { return b, nil }

type Getter interface {
	Get() T
	set(U)
}

// Anon is an unnamed interface that other packages use too.
func Anon() interface{ Make(T) (U, error) } { return nil }
