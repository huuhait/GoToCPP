// phase: 08
// canary: tsan

// A canary for the ThreadSanitizer job (step 15.11, tools/sanitize; the same program as
// tests/race/race_counter.gp): two goroutines add to a shared counter without synchronization.
// The job (gocpp -race) must report "WARNING: DATA RACE" for it.
package main

import "fmt"

var counter int

func work(done chan bool) {
	for i := 0; i < 100; i++ {
		counter++
	}
	done <- true
}

func main() {
	done := make(chan bool)
	go work(done)
	go work(done)
	<-done
	<-done
	fmt.Println("done")
}
