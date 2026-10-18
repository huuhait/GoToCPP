// phase: 06

// Named containers over named structs in interfaces (the 5.6/5.9 review's miscompile): the descriptor of a container points at the descriptor of its element, whichever is declared first, when both are in one package (cont) and when the container (use) is in a package and its element in another: the descriptor objects are declared extern in every pkg.hpp before any definition uses them.

package main

import (
	"fmt"

	"example.com/cont/cont"
	"example.com/cont/use"
)

func main() {
	for i, b := range cont.Boxes() {
		_, isLener := b.(cont.Lener)
		fmt.Println(i, isLener)
	}
	a := cont.Boxes()
	fmt.Println(len(a[0].(cont.Items)), a[0].(cont.Items)[1].V, (*a[2].(cont.PB)).N)
	fmt.Println(a[3].(map[string]cont.Base)["x"].N, a[4].([2]cont.Base)[1].N, a[5].(cont.NamedMap)["k"].N, a[6].(cont.NamedArr)[0].N)
	fmt.Println(a[8].(cont.Tree).Kids[0].Name, len(a[7].(cont.A)), len(a[9].(cont.Graph)["a"]), a[10].(*cont.Node) != nil)
	var l cont.Lener = a[1].(cont.Stack)
	fmt.Println(l.Len())
	for i, b := range use.Boxes() {
		_, isLener := b.(cont.Lener)
		fmt.Println(i, isLener)
	}
	u := use.Boxes()
	fmt.Println(u[0].(use.Items).Len(), u[1].(use.Names)["n"].N, u[2].(use.Pair)[1].V, (*u[3].(use.Ptr)).N, u[4].(use.Nodes)[0].Name)
	_, ok := u[0].(cont.Items)
	fmt.Println(ok)
}

// Output:
// 0 false
// 1 true
// 2 false
// 3 false
// 4 false
// 5 false
// 6 false
// 7 false
// 8 false
// 9 false
// 10 false
// 11 true
// 12 false
// 2 2 7
// 5 2 9 3
// leaf 2 1 true
// 3
// 0 true
// 1 false
// 2 false
// 3 false
// 4 false
// 2 4 6 8 t
// false
