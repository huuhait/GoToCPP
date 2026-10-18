// phase: 05
// ExitCode: 2
// Stderr: ^panic: runtime error: invalid memory address or nil pointer dereference\n\[signal SIGSEGV: segmentation violation code=0x1 addr=0x[0-9a-f]+ pc=0x[0-9a-f]+\]\n\ngoroutine 1 \[running\]:\n

// A promoted field through an embedded pointer is read after the calls of its statement (gc
// reads memory late): the pointer that a call replaces is the one that is followed, in an
// argument list, an operator, a slice literal, a map store and an assignment's target; and a
// call that makes the pointer nil makes the read panic.
package main

import "fmt"

type Base struct{ n int }

type W struct {
	*Base
	tag string
}

var g = W{&Base{1}, "g"}

func bump() int { g.Base = &Base{100}; g.tag = "bumped"; return 1 }
func zero() int { g.Base = nil; return 0 }

func main() {
	fmt.Println(g.n, bump(), g.n)
	fmt.Println(g.tag, bump())
	x := g.n + bump()
	fmt.Println(x)
	g.Base = &Base{5}
	arr := []int{g.n, bump(), g.n}
	fmt.Println(arr)
	g.Base = &Base{7}
	m := map[string]int{"a": g.n}
	m["b"] = g.n + bump()
	fmt.Println(m["a"], m["b"])
	g.Base = &Base{9}
	g.n, _ = bump(), 0
	fmt.Println(g.n, g.Base.n)
	fmt.Println(g.n, zero())
}

// Output:
// 100 1 100
// bumped 1
// 101
// [100 1 100]
// 7 101
// 1 1
