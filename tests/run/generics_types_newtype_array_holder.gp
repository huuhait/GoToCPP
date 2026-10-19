// phase: 07

// An instance held by value through a generic array type (a gp::newtype over [3]Cell[T]), reached only through a slice of another generic type: the held instance is complete before its holder. main names only Hold: naming an inner instance there (Row[bool]{}) would put it in the package's own table and hide the ordering bug.
package main

import "fmt"

type Cell[T any] struct{ c T }
type Vec[T any] [3]Cell[T]
type Row[T any] struct{ v Vec[T] }
type Hold[T any] struct{ rows []Row[T] }

func main() {
	var h Hold[bool]
	fmt.Println(h)
}

// Output:
// {[]}
