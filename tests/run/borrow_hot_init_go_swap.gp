// phase: 08

// init starts the reader and performs all protected publications.
// Publication and owner acquisition/the entire read hold access. Three channel handshakes
// make the output deterministic; copy/lend decisions are tested separately without the mutex.
package main

import (
	"fmt"
	"sync"
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

var total int

func init() {
	go worker()
	for value := 1; value <= 3; value++ {
		candidate := &Config{k: value}
		access.Lock()
		cfg = candidate
		access.Unlock()
		requests <- true
		total += <-results
	}
	close(requests)
	<-finished
}

func main() { fmt.Println("done", total == 6) }

// Output:
// done true
