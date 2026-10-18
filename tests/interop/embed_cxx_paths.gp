// phase: 05
// ExitCode: 2
// Stderr: ^panic: runtime error: invalid memory address or nil pointer dereference\n\[signal SIGSEGV: segmentation violation code=0x1 addr=0x[0-9a-f]+ pc=0x[0-9a-f]+\]\n\ngoroutine 1 \[running\]:\n

// Promoted members of an embedded C/C++ type through an embedded pointer inside an embedded
// struct: reads, writes, address-of, a Go method of the embedded Go type next to the C++ ones,
// closures that capture the outer value, a deferred closure, and the nil pointer on the way,
// which panics with Go's message.
package main

import (
	"fmt"

	"./embed_cxx.h"
	"<vector>"
)

type P struct {
	embed_cxx.epoint
	name string
}

type Mid struct{ *P }
type Top struct {
	Mid
	k int
}

func (p *P) Shift(d int) { p.x += C.int(d) }

func grow(v *std.vector[int]) { v.push_back(9) }

func main() {
	var t Top
	t.P = &P{name: "n"}
	t.x = 2
	t.y = 3
	t.scale(2)
	fmt.Println(int(t.x), int(t.y), int(t.sum()), int(t.Mid.P.epoint.x))
	px := &t.x
	*px = 10
	fmt.Println(int(t.x), int(t.P.x))
	t.Shift(5)
	fmt.Println(int(t.x))
	defer func() { t.scale(2); fmt.Println("scaled", int(t.x)) }()
	defer fmt.Println("defer", int(t.sum()))
	func() {
		t.x += 1
		t.y++
	}()
	fmt.Println(int(t.x), int(t.y))
	if t.sum() > 3 && t.x == 16 {
		fmt.Println("cond")
	}
	type L struct{ embed_cxx.elabel }
	var ll L
	ll.append("abc")
	fmt.Println(int(ll.size()))
	f := func() int { return int(ll.size()) }
	ll.append("d")
	fmt.Println(f())
	var nilt Top
	defer func() { fmt.Println("recovered?") }()
	nilt.x = 1
}

// Output:
// 4 6 10 4
// 10 10
// 15
// 16 7
// cond
// 3
// 4
// recovered?
// defer 21
// scaled 32
