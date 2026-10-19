// phase: 07

// Package variables initialized with instances of generic types, in this package and in another, and init functions that use them.
package main

import (
	"fmt"

	"example.com/gtinit/reg"
)

var Names = reg.New[int, string]().Add(1, "a")

var Both = reg.Default.Keys()

type Point struct{ X, Y int }

var Points = reg.New[Point, bool]().Add(Point{1, 2}, true)

func init() {
	reg.Log = append(reg.Log, "main init")
	Names.Add(2, "b")
}

func main() {
	fmt.Println(Names.Keys(), Both, Points.Keys())
	fmt.Println(reg.Log)
	fmt.Printf("%T %T\n", Names, Points)
}

// Output:
// [1 2] [one two] [{1 2}]
// [new add add reg init new add new add main init add]
// *reg.Registry[int,string] *reg.Registry[main.Point,bool]
