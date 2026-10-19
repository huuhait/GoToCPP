// phase: 07

// An instance of a generic type of another package whose methods use that package's unexported types, constants, variables and functions: the text is lowered in the declaring package's namespace.
package main

import (
	"fmt"

	"example.com/gtpriv/store"
)

type Item struct{ N int }

func main() {
	s := store.New[Item]()
	fmt.Println(s.Put("a", Item{1}), s.Put("b", Item{2}))
	fmt.Println(s.Last())
	fmt.Println(s.Keys(), store.Counter())
	t := store.New[string]()
	fmt.Println(t.Put("x", "y"))
	fmt.Printf("%T %v %T\n", s, store.Raw(t), store.Raw(s))
	defer func() { fmt.Println("recovered:", recover()) }()
	t.Put("b", "")
	t.Put("c", "")
	t.Put("d", "")
}

// Output:
// <{1}><a> <{2}><b>
// {2} 2
// [a b] 2
// <y><x>
// *store.Store[main.Item] [{x y 3}] []store.entry[main.Item]
// recovered: full: d
