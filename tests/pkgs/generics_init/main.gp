// phase: 07

// Package variables initialized by calls of generic functions (in main and in another package), and init functions that call them.
package main

import (
	"fmt"

	"example.com/ginit/reg"
)

var A = reg.Note(10)

var B = reg.Map([]string{"x", "yy"}, func(s string) int { return len(s) })

var Cv = reg.Note[float64](1.5) + float64(A)

var table = map[string][]int{"k": reg.Map([]int{1}, func(i int) int { return i + 100 })}

func init() {
	reg.Note("main init 1")
}

func init() {
	reg.Note(table["k"])
}

func main() {
	fmt.Println(A, B, Cv, table)
	fmt.Println(reg.Doubled, reg.First)
	fmt.Println(reg.Log)
}


// Output:
// 10 [1 2] 11.5 map[k:[101]]
// [2 4 6] reg first
// [2 4 6 reg first reg init 10 1.5 main init 1 [101]]
