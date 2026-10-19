// phase: 07

// Instances with array, pointer and function type arguments, and instances inside arrays and structs.
package main

import "fmt"

type Cell[T any] struct{ v T }

type Board[T any] struct {
	cells [3][3]Cell[T]
	moves int
}

func (b *Board[T]) Set(r, c int, v T) {
	b.cells[r][c] = Cell[T]{v}
	b.moves++
}

func (b Board[T]) Get(r, c int) T { return b.cells[r][c].v }

func (b Board[T]) Row(r int) [3]T {
	var out [3]T
	for i := range out {
		out[i] = b.cells[r][i].v
	}
	return out
}

func main() {
	var b Board[rune]
	b.Set(0, 0, 'x')
	b.Set(1, 1, 'o')
	b.Set(0, 2, 'x')
	fmt.Println(b.Row(0), b.Get(1, 1), b.moves)
	var p Board[*int]
	n := 5
	p.Set(2, 2, &n)
	*p.Get(2, 2) = 6
	fmt.Println(n, p.Get(0, 0) == nil)
	var f Board[func() int]
	f.Set(0, 0, func() int { return 42 })
	fmt.Println(f.Get(0, 0)())
	arrs := Board[[2]string]{}
	arrs.Set(1, 0, [2]string{"a", "b"})
	fmt.Println(arrs.Get(1, 0), arrs.Row(1))
	fmt.Printf("%T\n", arrs)
}

// Output:
// [120 0 120] 111 3
// 6 true
// 42
// [a b] [[a b] [ ] [ ]]
// main.Board[[2]string]
