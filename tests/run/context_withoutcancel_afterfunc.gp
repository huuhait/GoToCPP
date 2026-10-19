// phase: 09

// WithoutCancel detaches from the parent's cancellation; AfterFunc on it never runs, and a stop
// reports that the function had not started.
package main

import (
	"context"
	"fmt"
	"time"
)

func main() {
	parent, cancel := context.WithCancel(context.WithValue(context.Background(), "k", "v"))
	detached := context.WithoutCancel(parent)
	ran := make(chan string, 2)
	stopDetached := context.AfterFunc(detached, func() { ran <- "detached" })
	stopParent := context.AfterFunc(parent, func() { ran <- "parent" })
	cancel()
	fmt.Println(<-ran)
	time.Sleep(10 * time.Millisecond)
	select {
	case s := <-ran:
		fmt.Println("unexpected", s)
	default:
		fmt.Println("detached did not run")
	}
	fmt.Println(stopDetached(), stopParent())
	fmt.Println(parent.Err(), detached.Err(), detached.Value("k"))
	d, ok := detached.Deadline()
	fmt.Println(d.IsZero(), ok)
}

// Output:
// parent
// detached did not run
// true false
// context canceled <nil> v
// true false
