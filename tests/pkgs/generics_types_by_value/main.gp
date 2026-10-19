// phase: 07

// A package's struct holds by value instances that another package declares (Row, Grid): they are complete before it, in every header that mentions it.
package main

import (
	"fmt"

	"example.com/gtbyvalue/a"
	"example.com/gtbyvalue/b"
)

func main() {
	m := b.New()
	fmt.Printf("%v %+v\n", m, m.G)
	var g a.Grid[bool]
	g.Cells[0].A.V = true
	fmt.Println(g)
	fmt.Printf("%T\n", m.B)
}

// Output:
// {{[{{3} {4}}]} {[{{} {}} {{} {s}}]} {{1} {2}}} {Cells:[{A:{V:} B:{V:}} {A:{V:} B:{V:s}}]}
// {[{{true} {false}} {{false} {false}}]}
// a.Board[int]
