// phase: 08

// LockOSThread in a goroutine and in main is harmless: the goroutine runs and ends normally, and
// the count of goroutines is right afterwards.
package main

import (
	"fmt"
	"runtime"
)

func main() {
	runtime.LockOSThread()
	done := make(chan string)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		done <- "locked goroutine ran"
	}()
	fmt.Println(<-done)
	runtime.UnlockOSThread()
	for runtime.NumGoroutine() != 1 {
		runtime.Gosched()
	}
	fmt.Println(runtime.NumGoroutine())
}

// Output:
// locked goroutine ran
// 1
