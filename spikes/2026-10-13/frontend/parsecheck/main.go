package main

import (
	"fmt"
	"go/parser"
	"go/token"
)

func main() {
	cases := []string{
		"var a std.array[int, 5]",
		"var a = std.array[int, 5]{}",
		"var a = std.array[int, 5]()",
		"var a = std.get[0](t)",
		"var a = C.arr[3]",
		"var a = std.vector[std.vector[int]]{}",
		"var a = std.map[std.string, int]{}",
		"type T = std.array[int, N]",
	}
	for _, c := range cases {
		_, err := parser.ParseFile(token.NewFileSet(), "x.gp", "package p\n"+c+"\n", 0)
		status := "OK"
		if err != nil {
			status = "ERR: " + err.Error()
		}
		fmt.Printf("%-42s %s\n", c, status)
	}
}
