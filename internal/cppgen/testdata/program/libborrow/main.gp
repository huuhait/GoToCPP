package main

import (
	"fmt"

	"example.com/libborrow/lib"
)

func main() {
	t := &lib.T{N: 1, Name: "x"}
	lib.Held = t
	fmt.Println(lib.Use(t), t.Get(), lib.Drop(t))
}
