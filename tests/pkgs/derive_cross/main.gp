// phase: 05

// Derived methods across packages (step 10.10): the methods of a type of another package are
// callable and are used by the generators of the packages that hold that type, also through an
// embedded field and a generic instance.

package main

import (
	"fmt"

	"example.com/derive/model"
	"example.com/derive/store"
)

func main() {
	a := model.Item{Kind: model.Book, Name: "go", Tags: []string{"x"}, Extra: map[string]int{"n": 1}}
	b := a.Clone()
	b.Tags[0] = "y"
	fmt.Println(a, b, a.Equal(b), a.Equal(a.Clone()), model.Kind(9))

	var st model.Stack[string]
	st.Push("p")
	st.Push("q")
	st2 := st.Clone()
	st2.Items[0] = "z"
	fmt.Println(st.Items, st2.Items, *st.Top, *st2.Top, st.Equal(st2), st.Equal(st.Clone()))
	fmt.Println(model.NewSecret(1, 2).Same(model.NewSecret(1, 2)), model.NewSecret(1).Same(model.NewSecret(2)))

	s := store.Shelf{Item: a, Next: &b, Others: []model.Item{a}, Kinds: map[model.Kind][]model.Item{model.Film: {b}}}
	s.Stack = st
	c := s.Clone()
	c.Item.Tags[0] = "changed"
	c.Next.Name = "other"
	c.Others[0].Extra["n"] = 5
	c.Kinds[model.Film][0].Tags[0] = "k"
	c.Stack.Items[0] = "!"
	fmt.Println(s.Equal(c), s.Equal(s.Clone()), s.Item.Tags, b.Name, a.Extra, b.Tags, st.Items)
	fmt.Println(store.Shelf{Item: a, Kinds: map[model.Kind][]model.Item{model.Game: nil}})
}

// Output:
// {Book go [x] map[n:1]} {Book go [y] map[n:1]} false true Kind(9)
// [p q] [z q] q q false true
// true false
// false true [x] go map[n:1] [y] [p q]
// {{Book go [x] map[n:1]} <nil> [] map[Game:[]] {[] <nil>}}
