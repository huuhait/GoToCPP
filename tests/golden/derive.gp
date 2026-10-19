// phase: 10

// #Derive (step 10.10, ADR-080): the methods are ordinary Go methods written by the compiler, so the
// C++ is what a hand-written Equal, Clone and String would give: no helper calls, a switch for the
// enum, == for flat fields, loops for slices and maps, and the number of an unknown value written
// without any package.

package main

import "fmt"

#Derive(String)
type Level int

const (
	Debug Level = iota
	Info
)

#Derive(Equal, Clone)
type Event struct {
	ID    int
	Name  string
	Level Level
	Tags  []string
	Attrs map[string]int
	Next  *Event
}

func main() {
	e := Event{ID: 1, Tags: []string{"a"}}
	c := e.Clone()
	fmt.Println(e.Equal(c), Info)
}
