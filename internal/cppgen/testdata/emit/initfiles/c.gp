package main

import "fmt"

var c = note("c", twice())

var _ = note("blank", 0)

func twice() int { return b * 2 }

func init() {
	fmt.Println("c.gp init", steps)
}
