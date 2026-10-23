// phase: 08
// defer and go of calls with an argument larger than 128 KiB (an 8 MiB array) and a sibling
// argument whose C++ copy runs code (ADR-086 rule 2: the arguments are collected into owning
// snapshots in source order). The snapshot of the array is a heap copy, gp::detail::big_arg,
// moved into the frame or the goroutine: a copy on the C++ stack (auto gp_t0 = big;) overflowed
// the main thread's 8 MiB stack. Every call sees the array as it was before the sibling's copy
// changed it (1, not 2).
package main

import (
	"./arguments.hpp"
	"example.com/borrowlarge/lib"
	"fmt"
)

type Foreign struct{ value arguments.Copy }

type T struct{ n int }

type arr [1 << 20]int // a named type over big's: the snapshot converts to it on the heap

// mid is larger than 128 KiB but fits the stack. An unnamed struct converts to it with
// gp::convert<mid>, a temporary on the C++ stack (D24), and then goes to the heap copy.
type mid struct {
	n int
	a [1 << 15]int
}

var anon struct {
	n int
	a [1 << 15]int
}

type I interface {
	val(a [1 << 20]int, _ Foreign)
	sendv(a [1 << 20]int, _ Foreign)
}

var big [1 << 20]int
var foreign Foreign
var done = make(chan int, 1)

func setup() {
	big[1], anon.n = 1, 1
	arguments.set(func() {
		big[1], anon.n = 2, 2
	})
}

func showMid(m mid, _ Foreign) {
	fmt.Println("defer unnamed struct", m.n)
}

func sendMid(m mid, _ Foreign) {
	done <- m.n
}

func (t T) valMid(m mid, _ Foreign) {
	fmt.Println("defer unnamed struct method", t.n, m.n)
}

func show(a [1 << 20]int, _ Foreign) {
	fmt.Println("defer", a[1])
}

func showCount(a [1 << 20]int, n int) {
	fmt.Println("defer count", a[1], n)
}

func send(a [1 << 20]int, _ Foreign) {
	done <- a[1]
}

func showNamed(a arr, _ Foreign) {
	fmt.Println("defer named", a[1])
}

func sendNamed(a arr, _ Foreign) {
	done <- a[1]
}

func (t *T) takeNamed(a arr, _ Foreign) {
	fmt.Println("defer method named", t.n, a[1])
}

func (t *T) take(a [1 << 20]int, _ Foreign) {
	fmt.Println("defer method", t.n, a[1])
}

func (t *T) sendp(a [1 << 20]int, _ Foreign) {
	done <- a[1]*10 + t.n
}

func (t T) val(a [1 << 20]int, _ Foreign) {
	fmt.Println("defer value method", t.n, a[1])
}

func (t T) sendv(a [1 << 20]int, _ Foreign) {
	done <- a[1]*10 + t.n
}

func deferCalls() {
	setup()
	func() {
		defer show(big, foreign)
		fmt.Println("defer source", big[1])
	}()
	setup()
	func() { defer showCount(big, len([]Foreign{foreign})) }()
	setup()
	func() { defer lib.Count("qualified defer", big, len([]Foreign{foreign})) }()
	setup()
	func() { defer showNamed(big, foreign) }()
	setup()
	func() { defer showMid(anon, foreign) }()
	setup()
	func() { defer T{n: 8}.valMid(anon, foreign) }()
	setup()
	fv := show
	func() { defer fv(big, foreign) }()
	setup()
	t := &T{n: 3}
	func() { defer t.take(big, foreign) }()
	setup()
	func() { defer t.takeNamed(big, foreign) }()
	setup()
	func() { defer t.val(big, foreign) }()
	setup()
	var i I = T{n: 5}
	func() { defer i.val(big, foreign) }()
	setup()
	func() {
		defer func(a [1 << 20]int, _ Foreign) { fmt.Println("defer literal", a[1]) }(big, foreign)
	}()
}

func goCalls() {
	setup()
	go send(big, foreign)
	fmt.Println("go", <-done, big[1])
	setup()
	go sendNamed(big, foreign)
	fmt.Println("go named", <-done, big[1])
	setup()
	go sendMid(anon, foreign)
	fmt.Println("go unnamed struct", <-done, anon.n)
	setup()
	go lib.Send(big, len([]Foreign{foreign}), done)
	fmt.Println("qualified go", <-done, big[1])
	setup()
	fv := send
	go fv(big, foreign)
	fmt.Println("go func value", <-done)
	setup()
	t := &T{n: 4}
	go t.sendp(big, foreign)
	fmt.Println("go method", <-done)
	setup()
	go t.sendv(big, foreign)
	fmt.Println("go value method", <-done)
	setup()
	var i I = T{n: 6}
	go i.sendv(big, foreign)
	fmt.Println("go interface method", <-done)
	setup()
	go func(a [1 << 20]int, _ Foreign) { done <- a[1] }(big, foreign)
	fmt.Println("go literal", <-done)
}

func main() {
	deferCalls()
	goCalls()
}

// Output:
// defer source 2
// defer 1
// defer count 1 1
// qualified defer 1 1
// defer named 1
// defer unnamed struct 1
// defer unnamed struct method 8 1
// defer 1
// defer method 3 1
// defer method named 3 1
// defer value method 3 1
// defer value method 5 1
// defer literal 1
// go 1 2
// go named 1 2
// go unnamed struct 1 2
// qualified go 11 2
// go func value 1
// go method 14
// go value method 14
// go interface method 16
// go literal 1
