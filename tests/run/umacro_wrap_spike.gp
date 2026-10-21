// phase: 08

// The wrap-macro spike of 2026-10-13 (spikes/2026-10-13/macros-v3-wrap) as a gocpp program (exit
// criterion of Phase 10: "wrap macros behave as in the spike"): Trace (start style) and VM (wrap
// style) on one function, Recover, InThread on four goroutines, and Once. The twin is the spike's
// own main.go with its macros written as functions (testdata/macro_twin/umacro_wrap_spike.go), and
// the expected output below is what real Go prints for it; the spike's output is the same.
package main

import (
	"errors"
	"fmt"
	"sync"
)

macro Trace() func() {
	fmt.Println("enter", #func())
	return func() { fmt.Println("exit ", #func()) }
}

macro VM() {
	fmt.Println("  vm enter:", #func())
	defer fmt.Println("  vm exit: ", #func())
	#body()
}

macro Recover() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("recovered in", #func()+":", r)
		}
	}()
	#body()
}

macro InThread() {
	done := make(chan struct{})
	go func() {
		defer close(done)
		#body()
	}()
	<-done
}

macro Once(o *sync.Once) {
	o.Do(func() { #body() })
}

#Trace()
#VM()
func secret(key string) (int, error) {
	if key == "" {
		return 0, errors.New("empty key")
	}
	defer fmt.Println("  secret: body defer")
	return len(key) * 42, nil
}

#Recover()
func risky(n int) int { return 10 / n }

#InThread()
func work(id int) string { return fmt.Sprint("job ", id, " done") }

var (
	setupOnce sync.Once
	calls     int
)

#Once(&setupOnce)
func setup() int {
	calls++
	return calls
}

func main() {
	fmt.Println(secret("key"))
	fmt.Println(secret(""))
	fmt.Println("risky(2) =", risky(2))
	fmt.Println("risky(0) =", risky(0))

	var wg sync.WaitGroup
	results := make([]string, 4)
	for i := range 4 {
		wg.Add(1)
		go func() { defer wg.Done(); results[i] = work(i) }()
	}
	wg.Wait()
	fmt.Println(results)

	fmt.Println("setup:", setup(), setup(), "calls:", calls)
}

// Output:
// enter main.secret
//   vm enter: main.secret
//   secret: body defer
//   vm exit:  main.secret
// exit  main.secret
// 126 <nil>
// enter main.secret
//   vm enter: main.secret
//   vm exit:  main.secret
// exit  main.secret
// 0 empty key
// risky(2) = 5
// recovered in main.risky: runtime error: integer divide by zero
// risky(0) = 0
// [job 0 done job 1 done job 2 done job 3 done]
// setup: 1 0 calls: 1
