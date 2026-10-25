// gcbench: parks
// about: park and wake T goroutines on channels, on select and in Sleep (the waits the GC engine wraps in a blocking section)
//
// The same file is valid Go and gp: tools/gcbench builds it as main.go with the go
// command and as main.gp with gocpp.
//
//	parks THREADS [-quiet]
//
// It prints one deterministic line, "ops N", the number of park operations the
// workers were asked to do (identical in Go and gp); without -quiet it also prints
// the elapsed time and the operations per second (not compared).
//
// The workers are paired (0 with 1, 2 with 3, ...) and pass a token over two
// unbuffered channels, first with plain sends and receives and then through
// select. Each round in a pair is two hand-offs (one each way). A single worker
// has nobody to hand a token to, so at THREADS=1 only the Sleep phase runs.
package main

import (
	"os"
	"time"
)

func atoi(s string) int {
	n := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < '0' || c > '9' {
			break
		}
		n = n*10 + int(c-'0')
	}
	return n
}

const (
	rounds = 20000 // hand-offs per worker per phase (channel, then select)
	sleeps = 200   // Sleep calls per worker
	// sleepFor is long enough to enter the runtime's wait path, short enough
	// that the Sleep phase does not dominate the run.
	sleepFor = 100 * time.Microsecond
)

func main() {
	threads := 1
	quiet := false
	pos := 0
	for _, a := range os.Args[1:] {
		if a == "-quiet" {
			quiet = true
			continue
		}
		if pos == 0 {
			threads = atoi(a)
		}
		pos++
	}
	if threads < 1 {
		threads = 1
	}

	start := time.Now()

	// chans[i] is worker i's incoming channel; a pair (2k, 2k+1) has chans[2k]
	// (the odd worker sends, the even worker receives) and chans[2k+1] (the
	// other way).
	chans := make([]chan int, threads)
	pairs := threads / 2
	for i := 0; i < 2*pairs; i++ {
		chans[i] = make(chan int)
	}
	// work runs one worker: the two channel phases when it has a partner, then
	// the Sleep phase. It returns the number of parks the worker did.
	work := func(g int) int {
		ops := 0
		if g/2 < pairs {
			in := chans[g]
			var out chan int
			if g%2 == 0 {
				out = chans[g+1]
			} else {
				out = chans[g-1]
			}
			if g%2 == 0 {
				// send, then receive
				for i := 0; i < rounds; i++ {
					out <- i
					<-in
				}
				for i := 0; i < rounds; i++ {
					out <- i
					select {
					case <-in:
					}
				}
			} else {
				// receive, then send
				for i := 0; i < rounds; i++ {
					v := <-in
					out <- v + 1
				}
				for i := 0; i < rounds; i++ {
					var v int
					select {
					case v = <-in:
					}
					out <- v + 1
				}
			}
			ops += 4 * rounds
		}
		for i := 0; i < sleeps; i++ {
			time.Sleep(sleepFor)
		}
		ops += sleeps
		return ops
	}

	ops := 0
	if threads == 1 {
		ops = work(0)
	} else {
		results := make(chan int, threads-1)
		for g := 1; g < threads; g++ {
			go func(g int) {
				results <- work(g)
			}(g)
		}
		ops = work(0)
		for g := 1; g < threads; g++ {
			ops += <-results
		}
	}
	elapsed := time.Since(start)

	println("ops", ops)
	if !quiet {
		ms := int64(elapsed) / 1000000
		if ms < 1 {
			ms = 1
		}
		println("elapsed_ms", ms)
		println("ops_per_s", int64(ops)*1000/ms)
	}
}
