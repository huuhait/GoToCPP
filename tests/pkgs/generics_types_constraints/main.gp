// phase: 07

// Type arguments of main that satisfy another package's constraints: methods of main's types are called from the instance, wrap-around arithmetic uses the type argument's width.
package main

import (
	"fmt"

	"example.com/gtcons/lib"
)

type ID int

func (i ID) String() string { return fmt.Sprintf("#%d", int(i)) }

type Name struct{ first, last string }

func (n *Name) String() string { return n.first + " " + n.last }

type Level int8

type Celsius float64

func main() {
	l := lib.NewLabeled(",", ID(1), ID(2), ID(3))
	fmt.Println(l.Join())
	p := lib.NewLabeled(" & ", &Name{"a", "b"}, &Name{"c", "d"})
	fmt.Println(p.Join())
	var acc lib.Acc[Level]
	acc.Add(100).Add(27).Add(1)
	fmt.Println(acc.Total(), acc.Average(2))
	f := lib.Acc[Celsius]{}
	f.Add(1.5).Add(2)
	fmt.Println(f.Total(), f.Average(2))
	idx := lib.Index[string, ID]{"k": 7}
	fmt.Println(idx.Names(), len(idx))
	fmt.Printf("%T %T\n", l, acc)
}

// Output:
// #1,#2,#3
// a b & c d
// -128 -64
// 3.5 1.75
// [#7] 1
// lib.Labeled[main.ID] lib.Acc[main.Level]
