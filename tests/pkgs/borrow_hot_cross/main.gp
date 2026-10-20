// phase: 08

// once.Do starts the reader through a function value of another package. Publication and
// complete owner acquisition/read use the same mutex; three handshakes need no timing stress.
package main

import (
	"fmt"
	"sync"

	"example.com/hotcross/bg"
)

type Config struct{ k int }

var access sync.Mutex
var cfg *Config
var requests = make(chan bool)
var results = make(chan int)
var finished = make(chan bool)
var once sync.Once

func sum(c *Config) int { return c.k }

func worker() {
	for range requests {
		access.Lock()
		value := sum(cfg)
		access.Unlock()
		results <- value
	}
}

func main() {
	// Hook is installed before startup and remains unchanged until the reader has returned.
	bg.Hook = func() {
		worker()
		close(finished)
	}
	once.Do(bg.StartWorker)

	total := 0
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
	fmt.Println("done", total == 6)
}

// Output:
// done true
