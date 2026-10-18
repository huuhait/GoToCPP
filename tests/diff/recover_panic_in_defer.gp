// phase: 05

package main

import "fmt"

func a() {
	defer func() {
		fmt.Println("a: recovered", recover())
	}()
	defer func() {
		panic("from defer")
	}()
	panic("original")
}

func b() {
	defer func() {
		fmt.Println("b: outer recovered", recover())
	}()
	defer func() {
		defer func() { fmt.Println("b: inner recovered", recover()) }()
		panic("inside deferred")
	}()
	panic("b original")
}

func c() {
	defer func() { fmt.Println("c: last deferred sees", recover()) }()
	defer func() { fmt.Println("c: second sees", recover()); panic("c second") }()
	defer func() { fmt.Println("c: first runs, no recover") }()
	panic("c first")
}

func main() {
	a()
	b()
	c()
	fmt.Println("done")
}
