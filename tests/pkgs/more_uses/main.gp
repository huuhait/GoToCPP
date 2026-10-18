// phase: 06

// More uses of types of another package: defined array, slice and struct types with methods (blank fields, a pointer receiver on a slice), variadic functions and the call with ..., results by name with defer, closures and function fields, errors as values, and a local variable named like an imported package.

package main

import (
	"fmt"

	"example.com/more/lib"
)

type wrapper struct {
	lib.Padded
	lib.Stack
}

func main() {
	var g lib.Grid
	g.Set(0, 0, 1)
	g.Set(1, 1, 2)
	g.Set(2, 2, 4)
	g2 := g
	g2.Set(0, 0, 100)
	fmt.Println(g.Trace(), g2.Trace(), len(g), g[1])
	for i, row := range g {
		fmt.Print(i, row, ";")
	}
	fmt.Println()
	p := lib.Padded{A: 1, B: "b"}
	q := lib.Padded{A: 1, B: "b"}
	fmt.Println(p == q, p)
	var s lib.Stack
	s.Push("a")
	s.Push("b")
	x, ok := s.Pop()
	fmt.Println(x, ok, len(s))
	w := wrapper{p, s}
	w.Push("z")
	fmt.Println(len(w.Stack), w.A)
	fmt.Println(lib.Sum(), lib.Sum(1, 2, 3), lib.Sum([]int{4, 5}...))
	fmt.Println(lib.Join("-", "a", "b", "c"), "|"+lib.Join("+")+"|")
	inc, get := lib.Counter()
	inc()
	inc()
	fmt.Println(get())
	b := lib.New("btn", func(s string) string { return "clicked " + s })
	fmt.Println(b.Click(), b.Click())
	b.OnClick = nil
	fmt.Println(b.Click())
	m := map[string]int{"a": 1, "b": 2}
	lib.Delete(m, "a")
	fmt.Println(m)
	err := lib.Fail(3)
	fmt.Println(err, lib.Fail(0) == nil)
	if e, ok := err.(*lib.Err); ok {
		fmt.Println(e.Code)
	}
	lib := lib.Sum(1, 1) // a variable named like the package
	fmt.Println(lib)
}

// Output:
// 7 106 3 [0 2 0]
// 0 [1 0 0];1 [0 2 0];2 [0 0 4];
// true {1 0 b [0 0]}
// b true 1
// 2 1
// 0 6 9
// a-b-c ||
// 2
// clicked btn1 clicked btn2
// nothing
// map[b:2]
// err 3 true
// 3
// 2
