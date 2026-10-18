// phase: 05

// An interface method called while a panic runs: a handler passed the recovered value, a method
// that recovers by itself, and a deferred call of a nil interface (steps 5.4/5.5 with 5.10).
package main

import "fmt"

type Handler interface{ Handle(r any) string }
type Guard interface{ Guard() }

type logger struct{ prefix string }

func (l logger) Handle(r any) string { return l.prefix + fmt.Sprint(r) }

type self struct{ name string }

func (s self) Guard() {
	if r := recover(); r != nil {
		fmt.Println(s.name, "guard recovered:", r)
	}
}

func run(h Handler, f func()) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println(h.Handle(r))
		}
	}()
	f()
}

func guarded(g Guard, f func()) {
	defer g.Guard()
	f()
}

func main() {
	run(logger{"log: "}, func() { panic("one") })
	run(logger{"L2: "}, func() { var p *int; _ = *p })
	guarded(self{"s1"}, func() { panic("two") })
	var g Guard = self{"s2"}
	guarded(g, func() { panic(fmt.Sprint("three")) })
	func() {
		defer func() { fmt.Println("outer recovered:", recover()) }()
		guarded(nil, func() { fmt.Println("no guard") })
	}()
}

// Output:
// log: one
// L2: runtime error: invalid memory address or nil pointer dereference
// s1 guard recovered: two
// s2 guard recovered: three
// outer recovered: runtime error: invalid memory address or nil pointer dereference
