// phase: 08

// Goroutines and C++ threads in one program (step 4.18): a goroutine calls a C++ function that
// takes a lock, a std::thread (made by C++, not by gocpp) calls a Go function that sends on a
// channel (the thread gets a goroutine record on first use), and a std::mutex protects a
// counter that both kinds of thread update. Results are counted, never timed.
package main

import (
	"fmt"

	"<mutex>"
	"<thread>"
	"<vector>"
)

var mu std.mutex
var shared int

func bump(n int) {
	for i := 0; i < n; i++ {
		mu.lock()
		shared++
		mu.unlock()
	}
}

func sendFrom(ch chan int, v int) {
	ch <- v
}

func main() {
	done := make(chan bool)
	for i := 0; i < 4; i++ {
		go func() {
			bump(500)
			done <- true
		}()
	}

	ts := std.vector[std.thread]{}
	for i := 0; i < 3; i++ {
		ts.emplace_back(bump, 500)
	}

	ch := make(chan int, 8)
	var t std.thread = std.thread(sendFrom, ch, 42)
	fmt.Println(<-ch)
	t.join()

	for i := range ts {
		ts[i].join()
	}
	for i := 0; i < 4; i++ {
		<-done
	}
	fmt.Println(shared)
}

// Output:
// 42
// 3500
