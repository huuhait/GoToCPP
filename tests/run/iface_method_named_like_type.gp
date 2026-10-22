// phase: 05

// An interface with a method named like a type it uses (the 5.4 bug of the audit row L226):
// interface{ Item() Item }, a named interface, and one with a method that takes the type.
package main

import "fmt"

type Item struct{ n int }

func (i Item) Item() Item { return Item{i.n + 1} }
func (i Item) Value() int { return i.n }

type Source interface{ Item() Item }

type Named interface {
	Value() int
	Item() Item
	Take(Item) Item
}

func (i Item) Take(o Item) Item { return Item{i.n + o.n} }

var x interface{ Item() Item }

func chain(s Source) Item { return s.Item().Item() }

func main() {
	fmt.Println(x == nil)
	x = Item{1}
	fmt.Println(x.Item())
	var s Source = Item{5}
	fmt.Println(chain(s))
	var n Named = Item{7}
	fmt.Println(n.Value(), n.Item(), n.Take(Item{3}))
	fmt.Printf("%T %T\n", x, n)
}

// Output:
// true
// {2}
// {7}
// 7 {8} {10}
// main.Item main.Item
