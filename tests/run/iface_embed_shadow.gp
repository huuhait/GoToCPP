// phase: 05

// The itab has the method that Go's selector rules pick: a method of the outer type hides the
// promoted one at any depth, the shallowest promoted one wins, and an embedded field hides a
// method of the same name from deeper down (the type then lacks the method).
package main

import "fmt"

type Namer interface{ Name() string }
type Kinder interface{ Kind() string }

type A struct{}

func (A) Name() string { return "A.Name" }
func (A) Kind() string { return "A.Kind" }

type B struct{ A }

func (B) Name() string { return "B.Name" }

type Mid struct {
	B
	Kind string // a field hides A.Kind: Mid has no method Kind
}

type D struct{ Mid }

func (D) Kind() string { return "D.Kind" } // D's own, at depth 0

type E struct {
	D
	B2 B
}

func name(n Namer) string { return n.Name() }

func main() {
	var n Namer = B{}
	fmt.Println(n.Name(), name(Mid{}), name(D{}), name(E{}))
	var k Kinder = D{}
	fmt.Println(k.Kind())
	var any1 any = Mid{}
	_, isKinder := any1.(Kinder)
	_, isNamer := any1.(Namer)
	fmt.Println(isKinder, isNamer, Mid{Kind: "field"}.Kind)
	any1 = E{}
	_, isKinder = any1.(Kinder)
	fmt.Println(isKinder, E{}.Kind(), E{}.Mid.Kind == "")
	any1 = A{}
	_, isKinder = any1.(Kinder)
	fmt.Println(isKinder)
}

// Output:
// B.Name B.Name B.Name B.Name
// D.Kind
// false true field
// true D.Kind true
// true
