// phase: 04

// <thread>, <mutex>, <atomic>: a std::thread running a Go function through a callback (the
// thread gets a goroutine record on first use), join, a std::mutex with lock_guard, an
// atomic counter, and hardware_concurrency (only its type is used: the value varies).
package main

import (
	"fmt"

	"<atomic>"
	"<mutex>"
	"<thread>"
)

var counter std.atomic[int]
var total int
var mu std.mutex

func work(n int) {
	for i := 0; i < n; i++ {
		counter.fetch_add(1)
		mu.lock()
		total++
		mu.unlock()
	}
}

func main() {
	t1 := std.thread(work, 1000)
	t2 := std.thread(work, 1000)
	t1.join()
	t2.join()
	fmt.Println(int(counter.load()), total, t1.joinable())

	{
		g := std.lock_guard[std.mutex](mu)
		total += 5
		_ = g
	}
	fmt.Println(total, std.thread.hardware_concurrency() >= 0)
}

// Output:
// 2000 2000 false
// 2005 true
