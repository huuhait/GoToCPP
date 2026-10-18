// phase: 05

// The forwarder of Make and Eat uses the type Item in its signature, and the outer struct has a field
// named Item, gp and std: the type is written qualified in the members that come after.
package main

import "fmt"

type Item struct{ id int }
type Base struct{ n int }

func (b Base) Make() Item            { return Item{b.n} }
func (b *Base) Eat(i Item, is []Item) int { return i.id + len(is) }

type Outer struct {
	Base
	Item int
	gp   int
	std  string
}

type Outer2 struct {
	Base
	Make2 func(Item) Item
	Items []Item
}

func main() {
	o := Outer{Base{3}, 4, 5, "s"}
	fmt.Println(o.Make(), o.Item, o.Eat(Item{1}, nil), o.gp, o.std)
	o2 := Outer2{Base: Base{2}}
	fmt.Println(o2.Make(), o2.Eat(Item{5}, o2.Items))
}

// Output:
// {3} 4 1 5 s
// {2} 5
