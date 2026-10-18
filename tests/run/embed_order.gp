// phase: 05

// The receiver of a promoted method is evaluated with the arguments in gc's order: its calls
// before the arguments' calls, its reads after them; an embedded pointer is followed after the
// calls of the statement.
package main

import "fmt"

type Base struct{ n int }

func (b Base) Get() int    { return b.n }
func (b *Base) Add(k int)  { b.n += k }
func (b Base) Plus(k int) int { return b.n + k }

type W struct {
	*Base
	tag string
}

var g = W{&Base{1}, "g"}

func trace(s string, v int) int { fmt.Println("call", s); return v }

func bump() int { g.Base = &Base{100}; return 1 }

func recv() *W { fmt.Println("recv"); return &g }

func main() {
	// g.Plus(bump()): the receiver's read (the embedded pointer) comes after the call of bump.
	fmt.Println(g.Plus(bump()))
	// The receiver's call comes before the argument's.
	fmt.Println(recv().Plus(trace("arg", 5)))
	recv().Add(trace("add", 3))
	fmt.Println(g.n, g.Get())
	w := W{&Base{7}, "w"}
	fmt.Println(w.Plus(trace("a", 1)) + w.Plus(trace("b", 2)))
}

// Output:
// 101
// recv
// call arg
// 105
// recv
// call add
// 103 103
// call a
// call b
// 17
