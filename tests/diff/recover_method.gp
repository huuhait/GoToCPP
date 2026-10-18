// phase: 05

package main

import "fmt"

type Guard struct {
	name string
	hits int
}

func (g *Guard) Recover() {
	if r := recover(); r != nil {
		g.hits++
		fmt.Println(g.name, "recovered:", r)
	}
}

func (g Guard) ValueRecover() {
	if r := recover(); r != nil {
		fmt.Println(g.name, "value method recovered:", r)
	}
}

func run(g *Guard, msg string) {
	defer g.Recover()
	panic(msg)
}

func runValue(g Guard, msg string) {
	defer g.ValueRecover()
	panic(msg)
}

func runMethodValue(g *Guard, msg string) {
	f := g.Recover
	defer f()
	panic(msg)
}

func runMethodExpr(g *Guard, msg string) {
	defer (*Guard).Recover(g)
	panic(msg)
}

func main() {
	g := &Guard{name: "g"}
	run(g, "one")
	runValue(*g, "two")
	runMethodValue(g, "three")
	runMethodExpr(g, "four")
	fmt.Println("hits", g.hits)
}
