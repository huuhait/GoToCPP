// phase: 05

// recover in a method that is reached through a method value or a method expression of an
// interface (step 5.9, with 5.10's rules): the method value is a func value whose call is the
// deferred call, so recover works inside the dynamic method when the func value is deferred,
// and not when the method value is only called by another deferred function.
package main

import "fmt"

type Closer interface{ Close() }

type Guard struct{ name string }

func (g *Guard) Close() {
	if r := recover(); r != nil {
		fmt.Println(g.name, "recovered:", r)
	}
}

type Val struct{ name string }

func (v Val) Close() { fmt.Println(v.name, "sees:", recover()) }

type Plain struct{ name string }

func (p Plain) Close() { fmt.Println(p.name, "closed") }

type Holder struct {
	Closer
	n int
}

func deferValue(c Closer) {
	f := c.Close
	defer f()
	panic("by value")
}

func deferDirect(c Closer) {
	defer c.Close()
	panic("direct")
}

func deferBoundLater(c Closer) {
	f := c.Close
	c = Plain{"replaced"}
	defer f()
	defer c.Close()
	panic("bound later")
}

func deferExpr(c Closer) {
	defer Closer.Close(c)
	panic("method expression")
}

func deferExprValue(c Closer) {
	f := Closer.Close
	defer f(c)
	panic("method expression value")
}

func deferInClosure(c Closer) {
	defer func() {
		c.Close() // not the deferred call itself: recover returns nil
	}()
	panic("in closure")
}

func deferInClosureValue(c Closer) {
	f := c.Close
	defer func() { f() }()
	panic("closure calls method value")
}

func deferEmbedded(h Holder) {
	defer h.Close()
	panic("embedded call")
}

func deferEmbeddedValue(h Holder) {
	f := h.Close
	defer f()
	panic("embedded method value")
}

func deferSlice(cs []Closer) {
	fs := make([]func(), len(cs))
	for i, c := range cs {
		fs[i] = c.Close
	}
	defer fs[1]()
	defer fs[0]()
	panic("slice of method values")
}

func run(name string, f func()) {
	defer func() { fmt.Println(name, "->", recover()) }()
	f()
}

func main() {
	run("value", func() { deferValue(&Guard{"g1"}) })
	run("value val", func() { deferValue(Val{"v1"}) })
	run("direct", func() { deferDirect(&Guard{"g2"}) })
	run("bound later", func() { deferBoundLater(&Guard{"g3"}) })
	run("expr", func() { deferExpr(&Guard{"g4"}) })
	run("expr value", func() { deferExprValue(&Guard{"g5"}) })
	run("closure", func() { deferInClosure(&Guard{"g6"}) })
	run("closure value", func() { deferInClosureValue(&Guard{"g7"}) })
	run("embedded", func() { deferEmbedded(Holder{Closer: &Guard{"g8"}}) })
	run("embedded value", func() { deferEmbeddedValue(Holder{Closer: &Guard{"g9"}}) })
	run("slice", func() { deferSlice([]Closer{&Guard{"s0"}, &Guard{"s1"}}) })
	run("nothing", func() { f := Closer(&Guard{"calm"}).Close; f(); fmt.Println("no panic") })
}

// Output:
// g1 recovered: by value
// value -> <nil>
// v1 sees: by value
// value val -> <nil>
// g2 recovered: direct
// direct -> <nil>
// replaced closed
// g3 recovered: bound later
// bound later -> <nil>
// g4 recovered: method expression
// expr -> <nil>
// g5 recovered: method expression value
// expr value -> <nil>
// closure -> in closure
// closure value -> closure calls method value
// g8 recovered: embedded call
// embedded -> <nil>
// g9 recovered: embedded method value
// embedded value -> <nil>
// s0 recovered: slice of method values
// slice -> <nil>
// no panic
// nothing -> <nil>
