// phase: 05

// Promoted fields and methods on elements: slice and array elements (addressable), map values
// (not addressable: a value method and a field read only), range variables, and addresses of
// the embedded field of an element that keep the slice's array alive.
package main

import "fmt"

type Base struct{ n int }

func (b Base) Get() int   { return b.n }
func (b *Base) Inc() int  { b.n++; return b.n }

type Item struct {
	Base
	name string
}

type Ref struct {
	*Base
	name string
}

func main() {
	s := []Item{{Base{1}, "a"}, {Base{2}, "b"}}
	s[0].Inc()
	s[1].n += 10
	p := &s[1].Base
	p.Inc()
	s = nil // p keeps the array alive
	fmt.Println(p.n, p.Get())

	arr := [3]Item{{Base{1}, "x"}}
	arr[2].Inc()
	pa := &arr[0].n
	*pa = 50
	fmt.Println(arr[0].Get(), arr[2].n, arr)

	m := map[string]Item{"k": {Base{5}, "k"}}
	fmt.Println(m["k"].Get(), m["k"].n, m["zz"].Get(), m["zz"].n, m["k"].name)
	it := m["k"]
	it.Inc()
	m["k"] = it
	fmt.Println(m["k"].n)

	rm := map[string]Ref{"r": {&Base{1}, "r"}}
	rm["r"].Inc() // the embedded pointer is shared: no copy is needed
	rm["r"].n += 5
	fmt.Println(rm["r"].n, rm["r"].Get())

	items := []Item{{Base{1}, "a"}, {Base{2}, "b"}, {Base{3}, "c"}}
	total := 0
	for _, it := range items {
		it.Inc() // a copy
		total += it.n
	}
	for i := range items {
		items[i].Inc()
		total += items[i].n
	}
	fmt.Println(total, items)

	ptrs := []*Item{{Base{1}, "p"}, nil}
	ptrs[0].Inc()
	fmt.Println(ptrs[0].n, ptrs[1] == nil)

	ch := [2][]Item{{{Base{1}, "n1"}}, {{Base{2}, "n2"}}}
	ch[1][0].Inc()
	fmt.Println(ch[1][0].n, ch[0][0].Get())
}

// Output:
// 13 13
// 50 1 [{{50} x} {{0} } {{1} }]
// 5 5 0 0 k
// 6
// 7 7
// 18 [{{2} a} {{3} b} {{4} c}]
// 2 true
// 3 1
