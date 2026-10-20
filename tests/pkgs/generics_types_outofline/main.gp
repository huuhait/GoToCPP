// phase: 07

// Shared instances keep one set of out-of-line methods across packages and source files.
package main

import (
	"fmt"

	"example.com/outline/first"
	"example.com/outline/lib"
	"example.com/outline/second"
)

type local struct{ N int }

func main() {
	w := first.New()
	fmt.Println(w.First(), second.Last(w), again(w))
	second.Add(&w, 8)
	read := w.Reader()
	fmt.Println(w.Last(), read(), lib.Calls())

	var b lib.Box[local]
	b.Add(local{11})
	b.Add(local{13})
	fmt.Println(b.First(), b.Last(), b.Reader()(), lib.Calls())
}

// Output:
// 3 5 5
// 8 8 3
// {11} {13} {13} 5
