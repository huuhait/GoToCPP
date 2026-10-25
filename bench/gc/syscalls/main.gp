// gcbench: syscalls
// about: T goroutines call the syscalls of the runtime's time path (os.Getpid and time.Now) as fast as they can
//
// The same file is valid Go and gp: tools/gcbench builds it as main.go with the go
// command and as main.gp with gocpp.
//
//	syscalls THREADS [-quiet]
//
// It prints one deterministic line, "ops N", the number of calls the workers were
// asked to make (identical in Go and gp); without -quiet it also prints the elapsed
// time and the calls per second (not compared).
//
// os.Getpid is a real system call on Linux; time.Now is the runtime's clock read
// (clock_gettime through the vDSO on Linux), which the GC engine also has to make
// through its time native. Both are cheap enough to loop on, and neither result is
// printed: only that each call returned a sane value is counted.
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

const perWorker = 500000 // rounds of one Getpid and one time.Now each

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

	// work makes perWorker Getpid calls and perWorker time.Now calls and returns
	// how many returned a sane value (all of them, unless something is wrong).
	work := func() int {
		ok := 0
		for i := 0; i < perWorker; i++ {
			if os.Getpid() > 0 {
				ok++
			}
			t := time.Now()
			if t.Unix() > 0 {
				ok++
			}
		}
		return ok
	}

	start := time.Now()
	ops := 0
	if threads == 1 {
		ops = work()
	} else {
		results := make(chan int, threads-1)
		for g := 1; g < threads; g++ {
			go func() {
				results <- work()
			}()
		}
		ops = work()
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
