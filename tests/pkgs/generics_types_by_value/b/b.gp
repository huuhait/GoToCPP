package b

import "example.com/gtbyvalue/a"

// The package's own struct holds instances of a's types by value.
type Mine struct {
	B a.Board[int] // first: Board, then the Row and the Cell it needs, are found in this order
	G a.Grid[string]
	R a.Row[int]
}

func New() Mine {
	m := Mine{R: a.Row[int]{a.Cell[int]{1}, a.Cell[int]{2}}}
	m.G.Cells[1].B.V = "s"
	m.B.Add(3, 4)
	return m
}
