// phase: 08

// A pointer method receiver is loaded while its publication mutex is held.
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

func sum(c *Config) int      { return c.k }
func (c *Config) total() int { return sum(c) }

func worker() {
	defer close(finished)
	for range requests {
		access.Lock()
		value := cfg.total()
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
	go worker()
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
