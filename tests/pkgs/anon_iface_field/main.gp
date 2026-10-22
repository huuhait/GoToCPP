// phase: 06

// A struct field of an unnamed interface type (also in a package that only declares the type), in a package that never assigns into it (the
// definition of the interface struct must still come before the struct that holds it by value),
// and the same unnamed interface type in a second package.

package main

import (
	"fmt"

	"example.com/anonfield/a"
	"example.com/anonfield/b"
	"example.com/anonfield/types"
)

type Local struct {
	mu interface{ Lock() }
}

func main() {
	h := a.New()
	fmt.Println(h.Mu == nil, h.Items[0] == nil, h.Items[1] == nil, h.Cb == nil)
	h.Items[1] = a.Num(4)
	fmt.Println(h.Items[1].Get())
	var p b.Pair
	b.Lock(&p)
	h.Mu = p.First
	fmt.Println(b.Locked(p), h.Mu == p.Second)
	var l Local
	fmt.Println(l.mu == nil)
	l.mu = h.Mu
	fmt.Println(l.mu == p.First)
	var c types.Config
	fmt.Println(c.Names[0] == nil)
}

// Output:
// true true true true
// 4
// true true
// true
// true
// true
