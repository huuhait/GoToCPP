// phase: 07

// Instances of types whose names collide: a local type named like a generic's argument, same names in two scopes.
package main

import "fmt"

type Box[T any] struct{ v T }

type Item struct{ n int }

type item struct{ s string }

type Box2 = Box[Item]

func f() any {
	type Item2 = item
	return Box[Item2]{item{"lower"}}
}

func main() {
	var a Box[Item] = Box2{Item{1}}
	b := Box[item]{item{"x"}}
	fmt.Println(a, b)
	fmt.Printf("%T %T %T\n", a, b, f())
	var x any = a
	_, isBoxItem := x.(Box[Item])
	_, isBoxLower := x.(Box[item])
	_, lowerOK := f().(Box[item])
	fmt.Println(isBoxItem, isBoxLower, lowerOK)
}

// Output:
// {{1}} {{x}}
// main.Box[main.Item] main.Box[main.item] main.Box[main.item]
// true false true
