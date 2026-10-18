// phase: 05

// C pointers in interfaces ignore const and nesting (step 5.6): Go has no const, so a
// `const T*` that a C function returns is the dynamic type *T, the same as a `T*`, and a `T**`
// is **T whichever way it was made; two pointers to the same address are equal, and they are
// the same map key.
package main

import (
	"fmt"

	"./ifacehdr.hpp"
)

func main() {
	var a any = ifh.cname()
	var b any = ifh.mname()
	_, okA := a.(*C.char)
	_, okB := b.(*C.char)
	fmt.Println(okA, okB)

	var c any = ifh.cpt()
	var d any = ifh.mpt()
	_, okC := c.(*ifh.Pt)
	_, okD := d.(*ifh.Pt)
	fmt.Println(okC, okD, c == d)

	var cp *ifh.Pt = ifh.mpt()
	var f any = cp
	_, okF := f.(*ifh.Pt)
	fmt.Println(okF, c == f, d == f)

	var ci any = ifh.cint()
	var mi any = ifh.mint()
	pi, okI := ci.(*C.int)
	fmt.Println(okI, ci == mi, *pi)

	// Nested pointers.
	var e any = ifh.ppt()
	var g any = ifh.cppt()
	pp := ifh.ppt()
	var h any = pp
	_, okE := e.(**ifh.Pt)
	_, okG := g.(**ifh.Pt)
	_, okH := h.(**ifh.Pt)
	fmt.Println(okE, okG, okH, e == g, e == h)
	if q, ok := h.(**ifh.Pt); ok {
		fmt.Println((*q).x, (*q).y)
	}

	// Different levels are different types.
	_, single := e.(*ifh.Pt)
	_, double := c.(**ifh.Pt)
	fmt.Println(single, double)

	switch v := g.(type) {
	case *ifh.Pt:
		fmt.Println("single", v.x)
	case **ifh.Pt:
		fmt.Println("double", (*v).x)
	}

	// Pointers to the same address are one map key; a map keyed by any finds them by either route.
	m := map[any]string{}
	m[c] = "pt"
	m[ifh.mint()] = "int"
	m[ifh.cname()] = "name"
	fmt.Println(len(m), m[d], m[any(cp)], m[ci], m[mi], m[any(ifh.cint())])
	m[ifh.ppt()] = "pp"
	fmt.Println(len(m), m[g], m[h])
}

// Output:
// true true
// true true true
// true true true
// true true 5
// true true true true true
// 3 4
// false false
// double 3
// 3 pt pt int int int
// 4 pp pp
