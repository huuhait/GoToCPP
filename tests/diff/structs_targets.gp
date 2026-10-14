// phase: 03

// The two phases of an assignment whose targets are parts of a plain local variable (w.i, w.p,
// ww[0].p, a[0], w.arr[1]): Go evaluates the operands of every target (the index in s[w.i], the
// pointer of w.p.f) before it stores anything, also in a range loop and with a call of several
// results.
package main

import "fmt"

type T struct{ f int }

type W struct {
	i   int
	p   *T
	arr [2]int
}

func two() (int, int) { return 1, 3 }

func main() {
	var w W
	s := []int{0, 0}
	w.i, s[w.i] = 1, 3
	fmt.Println(s[0], s[1])
	w.i = 0
	s = []int{0, 0}
	w.i, s[w.i] = two()
	fmt.Println(s[0], s[1])
	w.i = 0
	s = []int{0, 0, 0, 0}
	for w.i, s[w.i] = range []int{5, 6} {
	}
	fmt.Println(s[0], s[1], s[2], s[3])
	old := &T{}
	w.p = old
	w.p, w.p.f = &T{}, 8
	fmt.Println(old.f, w.p.f)
	ww := [1]W{{p: &T{}}}
	ww[0].p, ww[0].p.f = nil, 4
	fmt.Println(ww[0].p == nil)
	a := [2]int{}
	x := [3]int{}
	a[0], x[a[0]] = 1, 2
	fmt.Println(a[0], a[1], x[0], x[1], x[2])
	w.arr[1], x[w.arr[1]] = 2, 9
	fmt.Println(x[0], x[1], x[2])
	pw := &w
	pw.i, s[pw.i] = 3, 7
	fmt.Println(s[0], s[1], s[2], s[3])
}
