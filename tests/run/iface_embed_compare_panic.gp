// phase: 05
// ExitCode: 2
// Stderr: ^panic: runtime error: comparing uncomparable type main\.Slicey\n\ngoroutine 1 \[running\]:\n

// == on structs with an embedded interface compares the dynamic values: two interfaces that hold the
// same uncomparable type panic, as in Go (a different dynamic type is just unequal).
package main

import "fmt"

type Namer interface{ Name() string }

type Slicey []int

func (s Slicey) Name() string { return "slice" }

type Dog struct{ n string }

func (d Dog) Name() string { return d.n }

type Pet struct {
	Namer
	age int
}

func main() {
	fmt.Println(Pet{Dog{"a"}, 1} == Pet{Slicey{1}, 1}) // different dynamic types: false
	fmt.Println(Pet{Slicey{1}, 1} == Pet{Dog{"a"}, 1})
	fmt.Println(Pet{Slicey{1}, 1} == Pet{Slicey{1}, 2}) // the first fields are compared first: panics
}

// Output:
// false
// false
