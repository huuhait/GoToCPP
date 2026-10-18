// The forwarders of Outer are marked late, by the method table that the conversion to Taker makes
// (no AllForwarders): the names Item and Pair of the promoted methods still hide the types of that
// name in the struct (a field and a parameter of type Item after the forwarder Item), and in the
// bodies of Outer's own methods, which are written before the mark is known.
package main

import "fmt"

type Item struct{ id int }
type Pair struct{ a, b int }
type Base struct{ n int }

func (b Base) Item() Item                { return Item{b.n} }
func (b Base) Pair() []Pair              { return []Pair{{b.n, b.n}} }
func (b *Base) Take(i Item, p *Pair) int { return i.id + p.a }

type Taker interface{ Take(Item, *Pair) int }

type Outer struct {
	Base
	Mine  Item
	Mine2 *Pair
	Arr   [2]Item
	F     func(Item) Pair
}

func (o Outer) Total() Item          { return Item{o.Mine.id + o.Base.Item().id} }
func (o *Outer) Use(i Item, p Pair) Pair { return Pair{i.id, p.a + o.Mine.id} }

func main() {
	o := &Outer{Base: Base{3}, Mine: Item{4}, Mine2: &Pair{1, 2}}
	var m Taker = o
	fmt.Println(o.Item(), m.Take(Item{1}, o.Mine2), o.Total(), o.Use(Item{1}, Pair{2, 3}))
}
