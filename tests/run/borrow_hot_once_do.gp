// phase: 09

// sort.Sort reaches Less, whose sync.Once callback starts the reader.
// Publication and owner acquisition/the entire read hold access. Three channel handshakes
// make the output deterministic; copy/lend decisions are tested separately without the mutex.
package main

import (
	"fmt"
	"sort"
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

func publish(value int) {
	candidate := &Config{k: value}
	access.Lock()
	cfg = candidate
	access.Unlock()
}

var once sync.Once

type sorter []int

func (s sorter) Len() int      { return len(s) }
func (s sorter) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s sorter) Less(i, j int) bool {
	once.Do(func() { go worker() })
	return s[i] < s[j]
}
func sortIt(s sorter) { sort.Sort(s) }

func main() {
	sortIt(sorter{3, 1, 2})
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
