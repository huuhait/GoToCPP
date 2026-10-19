// phase: 08

// The usual "recover and panic again" deferred function does nothing during Goexit: recover
// returns nil, so there is nothing to panic again with.
package main

import (
	"fmt"
	"runtime"
)

func guarded(name string) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println(name, "recovered", r, "and panics again")
			panic(r)
		}
		fmt.Println(name, "saw no panic")
	}()
}

func work(goexit bool) (result string) {
	defer func() {
		if r := recover(); r != nil {
			result = fmt.Sprint("outer recovered ", r)
		}
	}()
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("repanic:", r)
			panic(fmt.Sprint(r, " again"))
		}
	}()
	if goexit {
		runtime.Goexit()
	}
	panic("first")
}

func main() {
	fmt.Println(work(false))
	guarded("a")
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer fmt.Println("goroutine ends")
		work(true)
		fmt.Println("not reached")
	}()
	<-done
}

// Output:
// repanic: first
// outer recovered first again
// a saw no panic
// goroutine ends
