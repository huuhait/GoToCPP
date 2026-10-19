// phase: 07

// A value method called through a nil pointer: Go's text names the instance, main.Box[int].
package main

import "fmt"

type Box[T any] struct{ v T }

func (b Box[T]) Get() T { return b.v }

type Getter[T any] interface{ Get() T }

func main() {
	var p *Box[int]
	defer func() { fmt.Println("recovered:", recover()) }()
	var g Getter[int] = p
	fmt.Println(g.Get())
}

// Output:
// recovered: runtime error: invalid memory address or nil pointer dereference
