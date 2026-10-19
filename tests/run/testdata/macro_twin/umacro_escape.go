package main

import "fmt"

func Keep(f func() int) func() int {
	g := f
	return g
}

type Holder struct {
	fs []func() int
}

func (h *Holder) add(f func() int) { h.fs = append(h.fs, f) }

func (h *Holder) sum() (total int) {
	for _, f := range h.fs {
		total += f()
	}
	return total
}

func once() func() int {
	v := 7
	k := Keep(func() int { return v * 10 })
	v = 8
	return k
}

func shared() (get func() int, inc func()) {
	n := 0
	get = Keep(func() int { return n + 1 })
	inc = func() { n++ }
	return get, inc
}

func loop() *Holder {
	h := &Holder{}
	for i := 0; i < 3; i++ {
		h.add(Keep(func() int { return i * i }))
	}
	return h
}

func clobber() int {
	var a [64]int
	for i := range a {
		a[i] = -1
	}
	return a[3]
}

func main() {
	k := once()
	clobber()
	fmt.Println(k(), k())

	get, inc := shared()
	clobber()
	fmt.Println(get())
	inc()
	inc()
	fmt.Println(get())

	h := loop()
	clobber()
	fmt.Println(h.sum())
}
