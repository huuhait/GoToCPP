// gcbench: goroutines
// about: start N goroutines parked on a channel or running a recursion D frames deep (the cost of a goroutine and of the stop-the-world scan of its stack)
//
// The same file is valid Go and gp: tools/gcbench builds it as main.go with the go
// command and as main.gp with gocpp, so the two engines run the same algorithm.
//
//	goroutines -n N -mode blocked|running|mixed -depth D [-quiet]
//
// It prints two deterministic lines, "checksum <value>" and "goroutines N", with
// -quiet and without it; without -quiet it also prints the elapsed time (not
// compared). The checksum is over all N workers and depends on N and the mode; the
// depth enters it in running and mixed mode, where the workers recurse (blocked
// workers only register, so their checksum does not depend on the depth). The
// goroutine count is what runtime.NumGoroutine reports while every worker exists.
//
// N-1 goroutines are started; the main goroutine is the N-th worker. Every worker
// first sends on `started` and then blocks on `release`, so when the main goroutine
// has received all N-1 start messages every goroutine exists and is parked: that is
// where the goroutine count is read. In blocked mode the workers then stay parked
// for holdFor (so the harness can sample the peak resident set) and do nothing. In
// running mode they recurse depth frames and add churn to a sum. mixed alternates
// the two by the worker's index. Every worker contributes a deterministic value to
// the checksum, so a worker that skips its work changes it; a worker that never
// runs hangs the run, which the harness's timeout catches.
//
// Each recursion frame is a real call frame of the goroutine's stack: the harness's
// -goroutine-depth is the realistic stack depth whose scan a stop-the-world
// collector pays for (plan §2.5).
package main

import (
	"os"
	"runtime"
	"time"
)

const (
	// holdFor keeps the blocked workers parked long enough for the harness to
	// sample the peak resident set: its sampler needs a run of 20 ms.
	holdFor = 50 * time.Millisecond
	// busyRounds is the total recursion work of one run, split over the N
	// workers, so that every goroutine count does the same work. It is long
	// enough (about half a second in Go and one and a half in ARC on one
	// goroutine, and over 20 ms with sixteen processors at 1,000 goroutines)
	// for the harness's 20 ms resident-set sampler.
	busyRounds = 16000000
)

// atoi parses a non-negative decimal integer without importing strconv (the
// program should pull as little of the standard library as it can).
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

// churn recurses depth frames and returns a value that depends on the seed:
// the calls are the stack the collector has to scan.
func churn(depth, seed int) int {
	if depth <= 0 {
		return seed & 1023
	}
	return (churn(depth-1, seed) + 1) & 1023
}

func main() {
	n, depth := 1000, 32
	mode := "blocked"
	quiet := false
	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "-quiet" {
			quiet = true
		} else if a == "-n" && i+1 < len(args) {
			i++
			n = atoi(args[i])
		} else if a == "-mode" && i+1 < len(args) {
			i++
			mode = args[i]
		} else if a == "-depth" && i+1 < len(args) {
			i++
			depth = atoi(args[i])
		}
	}
	if n < 1 {
		n = 1
	}
	if depth < 1 {
		depth = 1
	}
	if mode != "running" && mode != "mixed" {
		mode = "blocked"
	}

	// busy reports whether worker g does the recursion; the rest only register.
	busy := func(g int) bool {
		if mode == "running" {
			return true
		}
		if mode == "mixed" {
			return g%2 == 1
		}
		return false
	}
	// register is the deterministic value of a worker that does not recurse.
	register := func(g int) int { return (g*7 + 1) & 1023 }

	rounds := busyRounds / n
	if rounds < 1 {
		rounds = 1
	}

	workers := n - 1
	started := make(chan int, workers)
	release := make(chan int, workers)
	results := make(chan int, workers)

	start := time.Now()
	for g := 1; g < n; g++ {
		go func(g int) {
			started <- g
			<-release
			if !busy(g) {
				results <- register(g)
				return
			}
			sum := 0
			for i := 0; i < rounds; i++ {
				sum += churn(depth, i+g)
			}
			results <- sum
		}(g)
	}
	for g := 1; g < n; g++ {
		<-started
	}
	// Every worker is parked on release: read the count the runtime reports.
	alive := runtime.NumGoroutine()
	if mode == "blocked" {
		time.Sleep(holdFor)
	}
	for g := 1; g < n; g++ {
		release <- g
	}
	sum := 0
	if busy(0) {
		for i := 0; i < rounds; i++ {
			sum += churn(depth, i)
		}
	} else {
		sum = register(0)
	}
	for g := 1; g < n; g++ {
		sum += <-results
	}
	elapsed := time.Since(start)

	println("checksum", sum)
	println("goroutines", alive)
	if !quiet {
		ms := int64(elapsed) / 1000000
		if ms < 1 {
			ms = 1
		}
		println("elapsed_ms", ms)
	}
}
