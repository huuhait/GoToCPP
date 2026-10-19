// phase: 08

// A locked Mutex is not tied to a goroutine: one goroutine may lock it and another unlock it.
package main

import (
	"fmt"
	"sync"
)

func main() {
	var mu sync.Mutex
	order := []string{}
	var log sync.Mutex
	add := func(s string) {
		log.Lock()
		order = append(order, s)
		log.Unlock()
	}

	mu.Lock()
	add("main locked")
	started := make(chan struct{})
	finished := make(chan struct{})
	go func() {
		close(started)
		mu.Lock() // waits for main's lock, which a third goroutine releases
		add("worker got the lock")
		mu.Unlock()
		close(finished)
	}()
	<-started
	unlocked := make(chan struct{})
	go func() {
		add("helper unlocks main's lock")
		mu.Unlock()
		close(unlocked)
	}()
	<-unlocked
	<-finished
	fmt.Println(len(order), order[0])
	fmt.Println("helper before worker:", indexOf(order, "helper unlocks main's lock") < indexOf(order, "worker got the lock"))
}

func indexOf(s []string, v string) int {
	for i, x := range s {
		if x == v {
			return i
		}
	}
	return -1
}

// Output:
// 3 main locked
// helper before worker: true
