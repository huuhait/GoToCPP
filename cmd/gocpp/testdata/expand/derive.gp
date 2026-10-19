// Derive shows the methods that #Derive writes.
package main

import "fmt"

// Color is an enum.
#Derive(String)
type Color int

const (
	Red Color = iota
	Green
	Blue
	Crimson = Red
)

// Order has slices, a map and a pointer.
#Derive(Equal, Clone)
type Order struct {
	ID    int
	Items []string
	Attrs map[string]int
	Owner *Person
}

type Person struct{ Name string }

#Derive(Equal, Clone)
type Person2 struct{ Name string }

func main() {
	o := Order{ID: 1, Items: []string{"a"}}
	fmt.Println(Green, o.Equal(o.Clone()))
}
