package b

import "example.com/gchain/a"

type Feet int

// Quad instantiates a's generic functions with b's own type and with its type parameter.
func Quad[T a.Number](x T) T { return a.Double(a.Double(x)) }

func FeetQuad(f Feet) Feet { return Quad(f) }

func Pipeline(x a.Meters) a.Meters { return a.Apply(x, a.Double[a.Meters], Quad[a.Meters]) }

func Add[T any](x T, add func(T) T) T { return a.Apply(x, add) }
