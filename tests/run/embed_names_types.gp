// phase: 05

// A promoted method named like a type (Item, Pair) next to fields, parameters and results of that type:
// the forwarder is a member of the outer struct, so a later use of the type name in the class must be
// written qualified (GCC: 'changes meaning of Item').
package main

import "fmt"

type Item struct{ id int }
type Pair struct{ a, b int }
type Base struct{ n int }

func (b Base) Item() Item { return Item{b.n} }
func (b Base) Pair() []Pair { return []Pair{{b.n, b.n}} }
func (b *Base) Take(i Item, p *Pair) int { return i.id + p.a }

type Outer struct {
	Base
	Mine  Item
	Mine2 *Pair
	Arr   [2]Item
	M     map[Item]Pair
	F     func(Item) Pair
}

func (o Outer) Total() Item { return Item{o.Mine.id + o.Item().id} }

func main() {
	o := Outer{Base: Base{3}, Mine: Item{4}, Mine2: &Pair{1, 2}}
	fmt.Println(o.Item(), o.Pair(), o.Take(Item{1}, o.Mine2), o.Total(), o.Arr, o.M == nil, o.F == nil)
}

// Output:
// {3} [{3 3}] 2 {7} [{0} {0}] true true
