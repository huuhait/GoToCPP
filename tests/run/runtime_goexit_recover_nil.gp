// phase: 08

// During runtime.Goexit, recover() returns nil: Goexit is not a panic, and nothing can stop it.
package main

import (
	"fmt"
	"runtime"
)

func main() {
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer func() {
			r := recover()
			fmt.Println("recover during Goexit:", r)
		}()
		defer func() {
			fmt.Println("recovering:", recover() == nil)
		}()
		runtime.Goexit()
	}()
	<-done
	fmt.Println("the goroutine is gone")
}

// Output:
// recovering: true
// recover during Goexit: <nil>
// the goroutine is gone
