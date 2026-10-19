// bench: embedding
// category: dispatch
// size: 100000000
// smoke: 100000
// twin: no
// about: promoted methods through struct embedding, directly and through an interface
package main

import "fmt"

type base struct{ n int }

func (b *base) inc()      { b.n++ }
func (b base) get() int   { return b.n }
func (b *base) add(k int) { b.n += k }

type mid struct {
	base
	k int
}

func (m *mid) add(k int) { m.base.add(k * 2) } // overrides the promoted method

type top struct {
	*mid
	extra int
}

type counter interface {
	inc()
	get() int
	add(k int)
}

func main() {
	t := top{mid: &mid{base: base{1}, k: 2}}
	var c counter = &t
	var m counter = t.mid
	var b counter = &base{5}
	var sum uint64
	for i := 0; i < benchN; i++ {
		t.inc()
		t.add(i & 3)
		sum += uint64(t.get())
		switch i % 3 {
		case 0:
			c.inc()
		case 1:
			m.add(1)
		default:
			b.add(i & 7)
		}
		sum += uint64(c.get()) + uint64(b.get())
	}
	fmt.Println(sum, t.n, t.mid.k, b.get())
}
