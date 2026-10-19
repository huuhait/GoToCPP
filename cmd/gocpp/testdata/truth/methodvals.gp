package main

import "fmt"

type T struct{ n int }

func (t *T) Inc()    { t.n++ }
func (t T) Get() int { return t.n }

func main() {
	t := T{1}
	f := t.Get
	g := t.Inc
	h := (*T).Inc
	g()
	h(&t)
	var iface interface{ Get() int } = T{3}
	fn := iface.Get
	fmt.Println(f(), t.n, fn())
}
