package main

import "fmt"

var trace []string

func f(name string, v int) int {
	trace = append(trace, name)
	return v
}

func main() {
	x := f("a", 1) + f("b", 2)*f("c", 3)
	m := map[string]int{"k": f("d", 4)}
	fmt.Println(x, m, trace)
}
