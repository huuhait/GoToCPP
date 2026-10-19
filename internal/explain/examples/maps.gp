package main

import "fmt"

func main() {
	m := map[string]int{"a": 1, "b": 2}
	m["c"] = 3
	m["c"]++
	v, ok := m["z"]
	delete(m, "a")
	fmt.Println(len(m), v, ok, m["b"])
}
