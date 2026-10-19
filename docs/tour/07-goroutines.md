# Goroutines and channels

`go f()` starts a goroutine, a channel carries values between goroutines, `select` waits for the
first of several channels, and `sync.WaitGroup` waits for a group of goroutines to finish.

> **Experimental today.** Goroutines, channels and `select` are phase 08. They work, but the
> compiler hides them until the phase is marked finished: run this lesson with
> `GOCPP_GATE_PHASE=08 gocpp run .`. Without the switch `gocpp check` refuses the program
> (below).

<!-- program: 08 id=workers go -->
```go
package main

import (
	"fmt"
	"sync"
	"time"
)

// square reads numbers from in and sends their squares to out.
func square(in <-chan int, out chan<- int) {
	for n := range in {
		out <- n * n
	}
	close(out)
}

func main() {
	in := make(chan int)
	out := make(chan int, 4) // buffered: sends do not wait for a receiver
	go square(in, out)

	go func() {
		for i := 1; i <= 4; i++ {
			in <- i
		}
		close(in)
	}()

	sum := 0
	for sq := range out {
		sum += sq
	}
	fmt.Println("sum of squares:", sum)

	// A WaitGroup waits for goroutines; a Mutex protects shared data.
	var (
		wg sync.WaitGroup
		mu sync.Mutex
	)
	total := 0
	for i := 1; i <= 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			mu.Lock()
			total += i
			mu.Unlock()
		}()
	}
	wg.Wait()
	fmt.Println("total:", total)

	// select takes the first channel that is ready.
	done := make(chan string)
	go func() {
		time.Sleep(20 * time.Millisecond)
		done <- "finished"
	}()
	select {
	case msg := <-done:
		fmt.Println(msg)
	case <-time.After(5 * time.Second):
		fmt.Println("timeout")
	}
}
```

<!-- output -->
```text
sum of squares: 30
total: 5050
finished
```

Without the switch the compiler refuses the program and names the phase:

<!-- rejected: workers -->
```text
./main.gp:10:13: channels are not supported yet (Phase 08)
```

## What to notice

- **A goroutine is an OS thread.** gocpp has no scheduler of its own: `go` starts a thread, a
  blocked goroutine blocks its thread, and the operating system does the scheduling. That is
  divergence [D7](../spec/language.md#d7-goroutines-are-os-threads) with its numbers: thousands
  of goroutines are fine, a million are not.
- Channels, `select`, `sync` and `time` follow Go's rules, including the Go 1.23 timer
  semantics. The programs of the test suite are compared with the real Go.
- A data race can break a reference count, and then it crashes where Go would only misbehave
  ([D10](../spec/language.md#d10-data-races-can-break-reference-counts)). `gocpp run -race`
  runs a race detector; see the [race guide](../guides/race.md).
- If every goroutine is blocked, gocpp reports `fatal error: all goroutines are asleep -
  deadlock!` like Go does.

Next: [The standard library](08-stdlib.md).
