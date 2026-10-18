// phase: 06

// Unnamed struct and interface types used in several packages are one type (Go's identity is structural), and unexported methods and fields of two packages are different (a.m and b.m): nothing here may be ambiguous in the C++.

package main

import (
	"fmt"

	"example.com/anon/a"
	"example.com/anon/b"
)

func main() {
	p := a.Pair()
	b.Take(p)
	fmt.Println(p.X, p.Y, p)
	w := a.Wrap(a.Named{3})
	fmt.Println(w.N.V, w)
	i := a.Ifc()
	b.Show(i)
	fmt.Println(i.M())
	var anyv interface{} = p
	_, ok := anyv.(struct {
		X int
		Y string
	})
	fmt.Println(ok)
	var iv interface{} = i
	_, ok = iv.(interface{ M() int })
	fmt.Println(ok)
	a.ShowHidden(a.Hidden())
	b.ShowHidden(b.Hidden())
	fmt.Println(a.CallM(a.MkT1()), b.CallM(b.MkT2()))
	var t1 interface{} = a.MkT1()
	_, ok = t1.(interface{ m() int })
	fmt.Println(ok)
}

// Output:
// b.Take 1 a
// 1 a {1 a}
// 3 {{3}}
// b.Show 5
// 5
// true
// true
// a.hidden 7
// b.hidden 8
// 1 2
// false
