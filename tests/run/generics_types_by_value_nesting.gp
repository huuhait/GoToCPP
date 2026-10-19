// phase: 07

// Instances of generic types that hold other instances by value (as fields, array elements, behind slices and maps): each struct is complete before the ones that hold it, whatever order the program mentions them in.
package main

import "fmt"

type Cell[T any] struct{ v T }
type Row[T any] struct{ a, b Cell[T] }
type Board[T any] struct{ rows []Row[T] }

func (b *Board[T]) Add(x, y T) { b.rows = append(b.rows, Row[T]{Cell[T]{x}, Cell[T]{y}}) }
func (b Board[T]) First() T     { return b.rows[0].a.v }

type W[T any] struct{ V T }

type Grid[T any] struct {
	cells [2][2]Cell[T]
	byRow map[int]Row[T]
}

func main() {
	var empty Board[int]
	fmt.Println(empty)
	var b Board[string]
	b.Add("x", "y")
	b.Add("z", "w")
	fmt.Println(b, b.First(), len(b.rows))
	fmt.Printf("%v %+v\n", b.rows[1], b.rows[1])

	var s W[[]W[[2]W[bool]]]
	s.V = append(s.V, W[[2]W[bool]]{[2]W[bool]{{true}, {false}}})
	fmt.Println(s, len(s.V))
	m := W[map[int]W[[2]W[bool]]]{map[int]W[[2]W[bool]]{7: {[2]W[bool]{{false}, {true}}}}}
	fmt.Println(m, m.V[7].V[1].V)

	g := Grid[float64]{byRow: map[int]Row[float64]{1: {Cell[float64]{1.5}, Cell[float64]{2.5}}}}
	g.cells[1][0] = Cell[float64]{3}
	fmt.Println(g.cells, g.byRow)
	fmt.Printf("%T\n%T\n", g, s)
	fmt.Printf("%T\n", W[struct {
		A int
		b string
	}]{})
}

// Output:
// {[]}
// {[{{x} {y}} {{z} {w}}]} x 2
// {{z} {w}} {a:{v:z} b:{v:w}}
// {[{[{true} {false}]}]} 1
// {map[7:{[{false} {true}]}]} true
// [[{0} {0}] [{3} {0}]] map[1:{{1.5} {2.5}}]
// main.Grid[float64]
// main.W[[]main.W[[2]main.W[bool]]]
// main.W[struct { A int; main.b string }]
