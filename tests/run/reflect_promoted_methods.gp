// phase: 09

package main

import (
	"fmt"
	"reflect"
)

type Animal struct{ Name string }

func (a Animal) Speak() string    { return a.Name }
func (a *Animal) Rename(n string) { a.Name = n }

type Dog struct {
	Animal
	Tricks int
}

func (d Dog) Fetch() string { return "fetch" }

type Puppy struct {
	*Dog
}

func main() {
	for _, v := range []any{Animal{}, &Animal{}, Dog{}, &Dog{}, Puppy{}, &Puppy{}} {
		t := reflect.TypeOf(v)
		fmt.Print(t, " ", t.NumMethod(), ":")
		for i := 0; i < t.NumMethod(); i++ {
			fmt.Print(" ", t.Method(i).Name)
		}
		fmt.Println()
	}
}

// Output:
// main.Animal 1: Speak
// *main.Animal 2: Rename Speak
// main.Dog 2: Fetch Speak
// *main.Dog 3: Fetch Rename Speak
// main.Puppy 3: Fetch Rename Speak
// *main.Puppy 3: Fetch Rename Speak
