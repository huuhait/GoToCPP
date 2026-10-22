// phase: 08

// recover in the deferred calls of goroutines, during a panic and during runtime.Goexit.
package main

import (
	"fmt"
	"runtime"
	"sync"
	"time"
)

func F() { recover() }

// G starts `go recover()`: the new goroutine is no deferred call of the panic, so it recovers
// nothing, whatever the ticket of G is (the sleep gives it time to run).
func G() {
	go recover()
	time.Sleep(20 * time.Millisecond)
}

func main() {
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer func() { fmt.Println("30 goroutine", recover()) }()
		panic("g1")
	}()
	wg.Wait()
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer func() { fmt.Println("31 goroutine outer", recover()) }()
		defer recover()
		panic("g2")
	}()
	wg.Wait()
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer func() { fmt.Println("32 goroutine F", recover()) }()
		defer F()
		panic("g3")
	}()
	wg.Wait()
	// Goexit
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer func() { fmt.Println("33 goexit recover", recover()) }()
		runtime.Goexit()
	}()
	wg.Wait()
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer func() { fmt.Println("34 goexit outer", recover()) }()
		defer recover()
		runtime.Goexit()
	}()
	wg.Wait()
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer func() { fmt.Println("35 goexit then panic in defer", recover()) }()
		defer func() { panic("g5") }()
		runtime.Goexit()
	}()
	wg.Wait()
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer func() { fmt.Println("36 goexit F", recover()) }()
		defer F()
		runtime.Goexit()
	}()
	wg.Wait()
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer func() { fmt.Println("37 go recover in a deferred function", recover()) }()
		defer G()
		panic("g6")
	}()
	wg.Wait()
}

// Output:
// 30 goroutine g1
// 31 goroutine outer g2
// 32 goroutine F <nil>
// 33 goexit recover <nil>
// 34 goexit outer <nil>
// 35 goexit then panic in defer g5
// 36 goexit F <nil>
// 37 go recover in a deferred function g6
