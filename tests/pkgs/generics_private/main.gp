// phase: 07

// Generic functions of another package use its unexported functions, variables, constants and types:
// the instances are written in main's files and name them with the package's namespace.
package main

import (
	"fmt"

	"example.com/gpriv/store"
)

func main() {
	fmt.Println(store.Put("a", 1), store.Put("a", "two"), store.Put("b", 3.5))
	fmt.Println(store.Calls(), store.Wrap(true, false, true))
	fmt.Println(store.Calls())
	store.Reset()
	fmt.Println(store.Calls(), store.Put("a", []int{1}))
	fmt.Printf("%v %T\n", store.Hidden(), store.Hidden())
	defer func() { fmt.Println("recovered:", recover()) }()
	for i := 0; i < 5; i++ {
		store.Put("z", i)
	}
}


// Output:
// a#1=1 a#2=two b#1=3.5
// 3 [w0#1=true w1#1=false w0#2=true]
// 6
// 0 a#1=[1]
// {h 1} store.entry
// recovered: too many z
