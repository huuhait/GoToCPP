// phase: 05

package main

import "fmt"

type T struct{ n int }

func (t T) Show(tag string)   { fmt.Println(tag, "value receiver", t.n) }
func (t *T) PShow(tag string) { fmt.Println(tag, "pointer receiver", t.n) }

func show(a, b int) { fmt.Println("show", a, b) }

func next(p *int) int {
	*p++
	return *p
}

func main() {
	t := T{1}
	defer t.Show("deferred")
	defer t.PShow("deferred")
	t.n = 2
	k := 0
	defer show(next(&k), next(&k))
	defer show(k, k)
	k = 100
	f := func() { fmt.Println("func value f") }
	defer f()
	f = func() { fmt.Println("func value g") }
	m := map[string]int{"a": 1, "b": 2}
	defer fmt.Println("len at defer:", len(m))
	defer delete(m, "a")
	defer func() { fmt.Println("m after delete:", m) }()
	defer println("builtin println")
	fmt.Println("body done", t.n, k)
}
