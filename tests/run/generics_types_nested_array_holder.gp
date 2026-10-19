// phase: 07

// An instance held by value through nested arrays ([2][3]Cell[T]), reached only through a slice of another generic type: the held instance is complete before its holder. main names only Hold: naming an inner instance there (G[int]{}) would put it in the package's own table and hide the ordering bug.
package main

import "fmt"

type Cell[T any] struct{ c T }
type G[T any] struct{ a [2][3]Cell[T] }
type Hold[T any] struct{ gs []G[T] }

func main() {
	var h Hold[int]
	fmt.Println(h)
}

// Output:
// {[]}
