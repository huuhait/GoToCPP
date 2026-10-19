package a

type Cell[T any] struct{ V T }
type Row[T any] struct{ A, B Cell[T] }
type Board[T any] struct{ Rows []Row[T] }
type Grid[T any] struct{ Cells [2]Row[T] }

func (b *Board[T]) Add(x, y T) { b.Rows = append(b.Rows, Row[T]{Cell[T]{x}, Cell[T]{y}}) }
