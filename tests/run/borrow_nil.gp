// phase: 08

// A nil pointer is a valid receiver and argument, also for a borrowed (raw) pointer: methods that
// test for nil work, and the first dereference panics with Go's message, in a direct call, a call
// through an interface, a promoted method, a method value, a method expression and a deferred
// call.
package main

import (
	"fmt"
	"runtime"
)

type Node struct {
	val  int
	next *Node
}

func (n *Node) IsNil() bool { return n == nil }

func (n *Node) Safe() int {
	if n == nil {
		return -1
	}
	return n.val
}

func (n *Node) Val() int { return n.val } // dereferences

func (n *Node) Sum() int {
	if n == nil {
		return 0
	}
	return n.val + n.next.Sum()
}

func read(n *Node) int { return n.val }

func same(a, b *Node) bool { return a == b }

type Valuer interface {
	Val() int
	Safe() int
}

type Wrap struct{ *Node }
type Wrap2 struct{ Wrap }

func try(name string, f func() int) {
	defer func() {
		r := recover()
		if e, ok := r.(runtime.Error); ok {
			fmt.Println(name, "panic:", e.Error())
		} else if r != nil {
			fmt.Println(name, "other panic:", r)
		}
	}()
	fmt.Println(name, f())
}

func main() {
	var n *Node
	fmt.Println(n.IsNil(), n.Safe(), n.Sum(), same(n, nil), same(n, &Node{}))
	try("method", func() int { return n.Val() })
	try("func", func() int { return read(n) })
	try("literal nil", func() int { return read(nil) })

	var v Valuer = n // a nil *Node in an interface is not a nil interface
	fmt.Println(v == nil, v.Safe())
	try("iface", func() int { return v.Val() })

	var w Wrap
	fmt.Println(w.IsNil(), w.Safe(), w.Sum())
	try("promoted", func() int { return w.Val() })
	var w2 Wrap2
	fmt.Println(w2.IsNil(), w2.Safe())
	try("promoted2", func() int { return w2.Val() })
	var pw *Wrap
	try("nil outer", func() int { return pw.Safe() })

	f := n.Safe // a method value of a nil receiver is fine
	g := n.Val
	fmt.Println(f())
	try("method value", g)
	h := (*Node).Safe
	k := (*Node).Val
	fmt.Println(h(n), h(&Node{val: 7}))
	try("method expr", func() int { return k(n) })

	try("deferred", func() (r int) {
		defer func() { r = -9 }()
		defer n.Val()
		return 1
	})
	fmt.Println("done")
}

// Output:
// true -1 0 true false
// method panic: runtime error: invalid memory address or nil pointer dereference
// func panic: runtime error: invalid memory address or nil pointer dereference
// literal nil panic: runtime error: invalid memory address or nil pointer dereference
// false -1
// iface panic: runtime error: invalid memory address or nil pointer dereference
// true -1 0
// promoted panic: runtime error: invalid memory address or nil pointer dereference
// true -1
// promoted2 panic: runtime error: invalid memory address or nil pointer dereference
// nil outer panic: runtime error: invalid memory address or nil pointer dereference
// -1
// method value panic: runtime error: invalid memory address or nil pointer dereference
// -1 7
// method expr panic: runtime error: invalid memory address or nil pointer dereference
// deferred panic: runtime error: invalid memory address or nil pointer dereference
// done
