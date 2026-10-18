// phase: 06

// An unnamed struct embedding a type of package b is shared by packages a (its owner: it writes its forwarders in a's pkg_desc.cpp), c (the only one that converts it to interfaces) and main.

package main

import (
	"fmt"

	"example.com/anonfwd/a"
	"example.com/anonfwd/b"
	"example.com/anonfwd/c"
)

func main() {
	g, i, v := c.Conv(3)
	i.Inc()
	fmt.Println(g.Get(), a.Show(a.Make(4)), v.(c.Getter).Get())
	w := struct{ b.T }{b.T{N: 1}}
	w.Inc()
	fmt.Println(w.Get(), a.Show(w))
	_, ok := v.(c.Incer)
	fmt.Println(ok)
}

// Output:
// 3 4 7
// 2 2
// false
