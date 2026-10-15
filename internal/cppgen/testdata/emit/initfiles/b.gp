package main

import "fmt"

var b = note("b", 1)

var steps int

func note(name string, v int) int {
	steps++
	fmt.Println("var", name, "=", v, "step", steps)
	return v
}

func init() {
	fmt.Println("b.gp init")
}
