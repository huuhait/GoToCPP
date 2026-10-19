package main

import "fmt"

type Animal struct{ name string }

func (a Animal) Name() string { return a.name }

type Dog struct {
	Animal
	tricks int
}

func main() {
	d := Dog{Animal{"rex"}, 2}
	fmt.Println(d.Name(), d.name, d.tricks)
}
