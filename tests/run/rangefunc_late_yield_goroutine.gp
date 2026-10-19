// phase: 09

// The loop ended, then several goroutines call the kept yield function at the same time. Each call
// must panic with Go's message in its goroutine (which recovers it) and run nothing of the body;
// the program stays clean under ASan and TSan. (The first call to fail sets the state to "panic", so
// a goroutine that is a little later may see either state: the test checks the common prefix.)
package main

import (
	"fmt"
	"strings"
	"sync"
)

func main() {
	var saved func(int) bool
	total := 0
	seq := func(yield func(int) bool) {
		saved = yield
		yield(1)
		yield(2)
	}
	for x := range seq {
		total += x
	}
	fmt.Println("total", total)

	var wg sync.WaitGroup
	var mu sync.Mutex
	var report []string
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				r := recover()
				mu.Lock()
				report = append(report, fmt.Sprint(r))
				mu.Unlock()
			}()
			saved(100 + i)
		}()
	}
	wg.Wait()
	n := 0
	for _, r := range report {
		if strings.HasPrefix(r, "runtime error: range function continued iteration after ") {
			n++
		}
	}
	fmt.Println(len(report), n)
	fmt.Println("total", total)
}

// Output:
// total 3
// 8 8
// total 3
