// phase: 05

// Go's depth rule with an embedded C struct: a Go field one level above the C struct's members wins
// (t.x is Inner.x, the C struct's x is two levels down), the C struct's members that no Go member
// hides are promoted through the embedded Go struct, and naming the field reaches the hidden one.
// (The same or a shallower depth is a gate: tests/errors/embed_cxx_shadow.gp.)
package main

import (
	"fmt"

	"./embed_cxx.h"
)

type Inner struct{ x string }
type Mid struct{ embed_cxx.epoint }

type T struct {
	Mid
	Inner
}

func main() {
	var t T
	t.x = "go"
	t.Mid.epoint.x = 3
	t.y = 4 // only the C struct has y
	fmt.Println(t.x, int(t.Mid.x), int(t.y), int(t.sum()))
	t.scale(2)
	fmt.Println(t.x, int(t.epoint.x), int(t.epoint.y))
}

// Output:
// go 3 4 7
// go 6 8
