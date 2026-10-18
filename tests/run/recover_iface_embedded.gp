// phase: 05

package main

import "fmt"

type Closer interface{ Close() }

type Guard struct{ name string }

func (g *Guard) Close() {
	if r := recover(); r != nil {
		fmt.Println(g.name, "recovered:", r)
	}
}

type ValueGuard struct{ name string }

func (v ValueGuard) Close() {
	fmt.Println(v.name, "value guard sees:", recover())
}

// Promoted methods: the method set of the outer type has Close through the embedded field.
type Outer struct {
	Guard
	extra int
}

type OuterPtr struct {
	*Guard
}

type OuterValue struct {
	ValueGuard
}

type Deep struct {
	Outer
}

type Wrapper struct {
	Closer // an embedded interface: the method is the dynamic type's
}

func run(c Closer, msg string) {
	defer c.Close()
	panic(msg)
}

func main() {
	defer func() { fmt.Println("main recovered:", recover()) }()
	run(&Outer{Guard: Guard{"outer"}}, "one")
	run(OuterPtr{&Guard{"outerptr"}}, "two")
	run(OuterValue{ValueGuard{"outervalue"}}, "three")
	run(&Deep{Outer{Guard: Guard{"deep"}}}, "four")
	run(Wrapper{&Guard{"wrapper"}}, "five")
	o := &Outer{Guard: Guard{"direct"}}
	func() {
		defer o.Close() // a promoted method called statically
		panic("six")
	}()
	panic("last")
}

// Output:
// outer recovered: one
// outerptr recovered: two
// outervalue value guard sees: three
// deep recovered: four
// wrapper recovered: five
// direct recovered: six
// main recovered: last
