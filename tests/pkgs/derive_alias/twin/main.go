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
