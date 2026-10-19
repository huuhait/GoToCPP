// phase: 07

// An instance of a generic interface type of another package, implemented by an instance of a generic type of that package and by a type of main.
package main

import (
	"fmt"

	"example.com/gtiface/lib"
)

type Const struct{ n int }

func (c Const) Get() int { return c.n }

type Counter struct{ n int }

func (c *Counter) Get() int  { return c.n }
func (c *Counter) Set(v int) { c.n = v }

func main() {
	a, b := lib.NewBox(1), lib.NewBox(2)
	lib.Swap(a, b)
	fmt.Println(a.Get(), b.Get(), lib.Read[int](a), lib.Read[int](Const{7}))
	c := &Counter{5}
	lib.Swap[int](a, c)
	fmt.Println(a.Get(), c.n)
	s := lib.NewBox("s")
	fmt.Printf("%T %T %v\n", a, s, s.Get())
	var g lib.Getter[int] = Const{1}
	_, isCell := g.(lib.Cell[int])
	_, isSetter := any(c).(lib.Setter[int])
	_, wrong := any(c).(lib.Setter[string])
	fmt.Println(isCell, isSetter, wrong)
}

// Output:
// 2 1 2 7
// 5 2
// *lib.Box[int] *lib.Box[string] s
// false true false
