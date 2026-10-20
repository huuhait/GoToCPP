// phase: 08

// time.AfterFunc starts the reader; the handshake needs no timer delay.
// Publication and owner acquisition/the entire read hold access. Three channel handshakes
// make the output deterministic; copy/lend decisions are tested separately without the mutex.
package main

import (
	"fmt"
	"sync"
	"time"
)

type Config struct{ k int }

var access sync.Mutex
var requests = make(chan bool)
var results = make(chan int)
var finished = make(chan bool)
var cfg *Config

func sum(c *Config) int { return c.k }

func worker() {
	defer close(finished)
	for range requests {
		access.Lock()
		value := sum(cfg)
		access.Unlock()
		results <- value
	}
}

func publish(value int) {
	candidate := &Config{k: value}
	access.Lock()
	cfg = candidate
	access.Unlock()
}

func main() {
	time.AfterFunc(0, worker)
	total := 0
	for value := 1; value <= 3; value++ {
		publish(value)
		requests <- true
		total += <-results
	}
	close(requests)
	<-finished

	fmt.Println("done", total == 6)
}

// Output:
// done true
