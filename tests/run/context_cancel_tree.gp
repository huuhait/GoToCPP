// phase: 09

// Cancelling a context cancels its whole subtree and nothing above or beside it; cancel is idempotent;
// a child of a cancelled parent is cancelled at once.
package main

import (
	"context"
	"errors"
	"fmt"
)

func state(name string, c context.Context) {
	select {
	case <-c.Done():
		fmt.Println(name, "done:", c.Err())
	default:
		fmt.Println(name, "live:", c.Err())
	}
}

func main() {
	root, cancelRoot := context.WithCancel(context.Background())
	a, cancelA := context.WithCancel(root)
	b, cancelB := context.WithCancel(root)
	a1, cancelA1 := context.WithCancel(a)
	a2, cancelA2 := context.WithCancel(a)
	defer cancelB()
	defer cancelA1()
	defer cancelA2()

	cancelA()
	<-a.Done()
	<-a1.Done()
	<-a2.Done()
	state("root", root)
	state("a", a)
	state("a1", a1)
	state("a2", a2)
	state("b", b)

	cancelA() // a second cancel changes nothing
	cancelA1()
	fmt.Println(a.Err() == context.Canceled, errors.Is(a1.Err(), context.Canceled))

	cancelRoot()
	<-b.Done()
	state("b", b)
	state("root", root)

	c, cancelC := context.WithCancel(root)
	defer cancelC()
	state("c (child of a cancelled parent)", c)
	fmt.Println(context.Cause(c) == context.Canceled)
}

// Output:
// root live: <nil>
// a done: context canceled
// a1 done: context canceled
// a2 done: context canceled
// b live: <nil>
// true true
// b done: context canceled
// root done: context canceled
// c (child of a cancelled parent) done: context canceled
// true
