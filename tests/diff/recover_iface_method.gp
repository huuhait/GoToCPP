// phase: 05

package main

import "fmt"

type Closer interface{ Close() }

type Guard struct{ name string }

// Close calls recover itself: through an interface it is still the deferred function.
func (g *Guard) Close() {
	if r := recover(); r != nil {
		fmt.Println(g.name, "recovered:", r)
	}
}

type Value struct{ name string }

func (v Value) Close() {
	fmt.Println(v.name, "value method sees:", recover())
}

type Quiet struct{ name string }

// Quiet.Close calls a function that recovers: not called by the deferred call, so nil.
func (q *Quiet) Close() {
	fmt.Println(q.name, "closing, helper sees:", func() any { return recover() }())
}

func run(c Closer, msg string) {
	defer c.Close()
	panic(msg)
}

func boundMethod(c Closer) {
	f := c.Close
	defer f()
	panic("bound")
}

func asFuncValue(c Closer) {
	fs := []func(){c.Close, (&Guard{"slice"}).Close}
	defer fs[1]()
	panic("slice element")
}

func main() {
	defer func() { fmt.Println("main recovered:", recover()) }()
	run(&Guard{"guard"}, "one")
	run(Value{"value"}, "two")
	func() {
		defer func() { fmt.Println("outer:", recover()) }()
		run(&Quiet{"quiet"}, "three")
	}()
	boundMethod(&Guard{"bound guard"})
	asFuncValue(&Guard{"unused"})
	var c Closer = &Guard{"normal call"}
	c.Close()
	fmt.Println("end")
	panic("last")
}
