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
