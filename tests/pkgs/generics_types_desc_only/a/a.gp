package a

type Box[E any] struct{ v E }

func (b Box[E]) Get() E { return b.v }

type T struct{ N int }

// Getter makes package a's header write interface wrappers.
type Getter interface{ Get() T }
