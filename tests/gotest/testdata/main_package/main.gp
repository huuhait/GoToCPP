package main

import "fmt"

type point struct{ X, Y int }

var inited = initial()

func initial() string { return "init" }

func init() { inited += "+init" }

func Add(a, b int) int { return a + b }

func main() {
	fmt.Println(Add(1, 2))
}
