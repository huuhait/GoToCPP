// phase: 08

// A negative counter is a panic with a string value that recover() returns.
package main

import (
	"fmt"
	"sync"
)

func try(f func()) {
	defer func() {
		r := recover()
		fmt.Printf("%T %v\n", r, r)
	}()
	f()
}

func main() {
	var wg sync.WaitGroup
	try(func() { wg.Done() })
	var wg2 sync.WaitGroup
	wg2.Add(2)
	try(func() { wg2.Add(-3) })
	var wg3 sync.WaitGroup
	wg3.Add(1)
	wg3.Done()
	wg3.Wait()
	fmt.Println("balanced add and done are fine")
}

// Output:
// string sync: negative WaitGroup counter
// string sync: negative WaitGroup counter
// balanced add and done are fine
