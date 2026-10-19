// phase: 07

// A builder with chained pointer methods that return the instance: x.A().B().C().
package main

import "fmt"

type Builder[T any] struct {
	items []T
	sep   string
	limit int
}

func New[T any]() *Builder[T] { return &Builder[T]{sep: ","} }

func (b *Builder[T]) Add(vs ...T) *Builder[T] {
	b.items = append(b.items, vs...)
	return b
}

func (b *Builder[T]) Limit(n int) *Builder[T] {
	b.limit = n
	return b
}

func (b *Builder[T]) Sep(s string) *Builder[T] {
	b.sep = s
	return b
}

func (b *Builder[T]) Build() string {
	out := ""
	for i, it := range b.items {
		if b.limit > 0 && i >= b.limit {
			break
		}
		if i > 0 {
			out += b.sep
		}
		out += fmt.Sprint(it)
	}
	return out
}

func main() {
	fmt.Println(New[int]().Add(1, 2, 3).Sep("-").Build())
	fmt.Println(New[string]().Add("a").Add("b", "c").Limit(2).Build())
	fmt.Println(New[float64]().Add(0.5).Sep(";").Limit(1).Build() == "0.5")
	b := New[bool]()
	b.Add(true).Add(false)
	fmt.Println(b.Build(), len(b.items))
}

// Output:
// 1-2-3
// a,b
// true
// true,false 2
