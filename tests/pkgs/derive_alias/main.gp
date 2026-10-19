// phase: 10

// An alias to a composite type of another package, used by a file that does not import that package
// (step 10.10): the generated code names the alias, not the type behind it.

package main

import (
	"fmt"

	"example.com/pk/model"
)

func main() {
	b := Box{A: Things{{1, "a"}, {2, "b"}}, T: Table{"k": {{9, "x"}}}, Names: []string{"n"}}
	c := b.Clone()
	c.A[0].Name = "changed"
	c.T["k"][0].Name = "y"
	fmt.Println(b.A, b.T, c.A, c.T, b.Equal(c))
	c.A[0].Name = "a"
	c.T["k"][0].Name = "x"
	fmt.Println(b.Equal(c), b.Equal(Box{A: Things{model.Thing{N: 1}, model.Thing{N: 3}}}))
}

// Output:
// [{1 a} {2 b}] map[k:[{9 x}]] [{1 changed} {2 b}] map[k:[{9 y}]] true
// true false
