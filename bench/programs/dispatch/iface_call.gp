// bench: iface_call
// category: dispatch
// size: 150000000
// smoke: 100000
// twin: yes
// about: interface method calls in a hot loop, three implementations picked pseudo-randomly
package main

import "fmt"

type shape interface {
	area() int
	grow(k int)
}

type square struct{ s int }
type rect struct{ w, h int }
type tri struct{ b, h, pad int }

func (q *square) area() int  { return q.s * q.s }
func (q *square) grow(k int) { q.s += k & 1 }
func (r *rect) area() int    { return r.w * r.h }
func (r *rect) grow(k int)   { r.h += k & 1 }
func (t *tri) area() int     { return t.b * t.h / 2 }
func (t *tri) grow(k int)    { t.b += k & 1 }

func main() {
	shapes := []shape{&square{3}, &rect{4, 5}, &tri{6, 7, 0}, &square{8}, &rect{2, 9}, &tri{10, 3, 0}, &square{1}, &rect{7, 7}}
	var idx [1024]int
	x := uint32(1)
	for i := range idx {
		x = x*1664525 + 1013904223
		idx[i] = int(x>>16) & 7
	}
	var sum uint64
	for i := 0; i < benchN; i++ {
		s := shapes[idx[i&1023]]
		sum += uint64(s.area())
		if i&255 == 0 {
			s.grow(i)
		}
	}
	fmt.Println(sum)
}
