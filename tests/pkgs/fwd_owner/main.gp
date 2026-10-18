// phase: 06

// A struct of package outer embeds a type of package inner and is converted to interfaces only in package use: the forwarders of its promoted methods and its descriptor are written by outer's pkg_desc.cpp, whoever asks for them.

package main

import (
	"fmt"

	"example.com/fwdowner/outer"
	"example.com/fwdowner/use"
)

func main() {
	w := outer.New(5)
	g, a, v := use.Convert(w)
	a.Add(10)
	fmt.Println(g.Get(), w.Get())
	gv, ok := v.(use.Getter)
	fmt.Println(gv.Get(), ok)
	_, ok = v.(use.Adder)
	fmt.Println(ok)
}

// Output:
// 15 15
// 5 true
// false
