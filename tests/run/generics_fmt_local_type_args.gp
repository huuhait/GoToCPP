// phase: 08

// A type declared inside a function used as a type argument: Go's %T numbers these types in the
// order of their declarations in the package (main.Box[main.point·1]): a number that counts every
// defined type declared in a function body (closures and generic functions too), whether or not
// it is used as an argument. A local type that is not inside brackets prints without it.
package main

import "fmt"

type Box[T any] struct{ V T }

type P struct{}

func Show[T any](v T) { fmt.Printf("%T %v %+v\n", v, v, v) }

func a() {
	type p int
	type q int
	alias := 0
	_ = alias
	fmt.Printf("%T %T %T\n", Box[p]{}, Box[q]{}, Box[P]{})
	func() {
		type p int
		fmt.Printf("%T\n", Box[p]{})
	}()
}

func b() {
	type r int
	type p int
	fmt.Printf("%T %T\n", Box[r]{}, Box[p]{})
	type al = r // an alias is not a new type
	fmt.Printf("%T\n", Box[al]{})
}

func G[T any]() {
	type loc struct{ v T }
	fmt.Printf("%T %T %T\n", loc{}, Box[loc]{}, []Box[loc]{})
}

func main() {
	type z int
	a()
	b()
	G[int]()
	G[string]()
	fmt.Printf("%T\n", Box[z]{})
	var i any = Box[z]{}
	_, ok := i.(Box[z])
	fmt.Println(ok)
	type point struct{ X, Y int }
	Show(point{1, 2})
	Show(Box[point]{point{3, 4}})
	Show([]point{{5, 6}})
	Show(map[z]point{1: {7, 8}})
	Show(&Box[Box[point]]{})
	Show(Box[func(z) point]{})
	Show(Box[chan z]{})
	Show(Box[[2]z]{})
	Show(Box[struct{ f z }]{})
}

// Output:
// main.Box[main.p·1] main.Box[main.q·2] main.Box[main.P]
// main.Box[main.p·3]
// main.Box[main.r·4] main.Box[main.p·5]
// main.Box[main.r·4]
// main.loc[int] main.Box[main.loc[int]·6] []main.Box[main.loc[int]·6]
// main.loc[string] main.Box[main.loc[string]·6] []main.Box[main.loc[string]·6]
// main.Box[main.z·7]
// true
// main.point {1 2} {X:1 Y:2}
// main.Box[main.point·8] {{3 4}} {V:{X:3 Y:4}}
// []main.point [{5 6}] [{X:5 Y:6}]
// map[main.z]main.point map[1:{7 8}] map[1:{X:7 Y:8}]
// *main.Box[main.Box[main.point·8]] &{{{0 0}}} &{V:{V:{X:0 Y:0}}}
// main.Box[func(main.z·7) main.point·8] {<nil>} {V:<nil>}
// main.Box[chan main.z·7] {<nil>} {V:<nil>}
// main.Box[[2]main.z·7] {[0 0]} {V:[0 0]}
// main.Box[struct { main.f main.z·7 }] {{0}} {V:{f:0}}
