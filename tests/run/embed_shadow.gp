// phase: 05

// Go's selector rules: the shallowest depth wins; a field of the outer type hides a method of an
// embedded one and the other way round, and a method the outer type declares hides the
// promoted ones; the explicit path always reaches the hidden one.
package main

import "fmt"

type Inner struct {
	Name string
	ID   int
}

func (Inner) Describe() string { return "inner" }
func (Inner) Kind() string     { return "inner kind" }
func (i Inner) Title() string  { return "inner title " + i.Name }

type Mid struct {
	Inner
	Describe string // a field hides the method Inner.Describe
	Level    int
}

func (m Mid) Kind() string { return "mid kind" }

type Top struct {
	Mid
	ID    string // hides Inner.ID (depth 2)
	Title func() string
}

func main() {
	t := Top{Mid{Inner{"in", 1}, "field", 2}, "top id", func() string { return "func field" }}
	fmt.Println(t.Describe, t.Mid.Inner.Describe(), t.Inner.Describe())
	fmt.Println(t.Kind(), t.Inner.Kind(), t.Mid.Kind())
	fmt.Println(t.ID, t.Inner.ID, t.Mid.ID, t.Mid.Inner.ID)
	fmt.Println(t.Title(), t.Inner.Title(), t.Name, t.Level)
	f := t.Kind
	g := Top.Kind
	h := Inner.Kind
	fmt.Println(f(), g(t), h(t.Inner))
}

// Output:
// field inner inner
// mid kind inner kind mid kind
// top id 1 1 1
// func field inner title in in 2
// mid kind mid kind inner kind
