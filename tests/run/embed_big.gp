// phase: 05

// An embedded struct larger than 128 KiB (boxed locals, parameters passed by reference): the
// promoted fields and methods still work, and copies are copies.
package main

import "fmt"

type Big struct {
	data [40000]int
	n    int
}

func (b *Big) Fill(v int) {
	for i := range b.data {
		b.data[i] = v + i
	}
	b.n = v
}
func (b Big) Sum() int {
	s := 0
	for _, x := range b.data {
		s += x
	}
	return s
}

type Holder struct {
	Big
	name string
}

type PHolder struct {
	*Big
	name string
}

func total(h Holder) int { h.Fill(1); return h.Sum() }

func main() {
	var h Holder
	h.Fill(2)
	cp := h
	cp.Fill(3)
	fmt.Println(h.Sum(), cp.Sum(), total(h), h.Sum(), h.n, cp.n, h.data[39999])
	ph := PHolder{&Big{}, "p"}
	ph.Fill(1)
	q := ph
	q.Fill(2)
	fmt.Println(ph.Sum(), q.Sum(), ph.n)
	hs := []Holder{{name: "a"}, {name: "b"}}
	hs[1].Fill(4)
	fmt.Println(hs[0].Sum(), hs[1].Sum(), hs[1].data[1])
}

// Output:
// 800060000 800100000 800020000 800060000 2 3 40001
// 800060000 800060000 2
// 0 800140000 5
