// phase: 07

// A type declared in a function used as a type argument of another package's generic type, nested in
// itself and in another instance: the instance has one identity however it is reached (the package that
// declares the generic type makes it, and numbers the local type as gc does: main.L·1).
package main

import (
	"fmt"

	"example.com/gtlocal/box"
)

type G[T any] struct{ x box.Box[box.Box[T]] }

func Make[T any](v T) G[T] { return G[T]{box.Wrap(v)} }

func main() {
	type L struct{ n int }
	type M struct{ s string }
	var x box.Box[box.Box[L]]
	x.V.V.n = 3
	fmt.Printf("%T %v\n", x, x)
	y := box.Wrap(L{4})
	fmt.Println(x == box.Box[box.Box[L]]{box.Box[L]{L{3}}}, y.Get().Get().n)
	g := Make(M{"m"})
	fmt.Printf("%T %v\n", g, g)
	var p box.Pair[box.Box[L], box.Box[M]]
	p.First.V.n, p.Second.V.s = 5, "six"
	fmt.Printf("%T %+v\n", p, p)
	var e any = box.Box[L]{L{1}}
	_, ok1 := e.(box.Box[L])
	_, ok2 := e.(box.Box[M])
	fmt.Println(ok1, ok2)
	func() {
		type L int
		var z box.Box[box.Box[L]]
		fmt.Printf("%T\n", z)
	}()
}

// Output:
// box.Box[example.com/gtlocal/box.Box[main.L·1]] {{{3}}}
// true 4
// main.G[main.M·2] {{{{m}}}}
// box.Pair[example.com/gtlocal/box.Box[main.L·1],example.com/gtlocal/box.Box[main.M·2]] {First:{V:{n:5}} Second:{V:{s:six}}}
// true false
// box.Box[example.com/gtlocal/box.Box[main.L·3]]
