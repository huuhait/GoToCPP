// phase: 08
// pending: steps 8.5 (select) and 8.7 (sync)

// Workers shows goroutines (each one is an OS thread in gocpp), buffered and
// unbuffered channels, a pipeline, select with a timeout and with default,
// sync.WaitGroup (Add, Done, Wait and Go), sync.Mutex and sync/atomic. The
// output is the same on every run: results are collected first, then printed
// in a fixed order.
package main

import (
	"fmt"
	"slices"
	"sync"
	"sync/atomic"
	"time"
)

type job struct {
	id, n int
}

type result struct {
	id, square int
}

// worker squares numbers until jobs is closed.
func worker(jobs <-chan job, results chan<- result, wg *sync.WaitGroup) {
	defer wg.Done()
	for j := range jobs {
		results <- result{j.id, j.n * j.n}
	}
}

// gen sends 1 to n on a new channel, then closes it.
func gen(n int) <-chan int {
	out := make(chan int)
	go func() {
		defer close(out)
		for i := 1; i <= n; i++ {
			out <- i
		}
	}()
	return out
}

// square sends the square of every number it reads.
func square(in <-chan int) <-chan int {
	out := make(chan int)
	go func() {
		defer close(out)
		for n := range in {
			out <- n * n
		}
	}()
	return out
}

func main() {
	// A pool of 4 workers for 9 jobs. Each result carries its job's id, so the
	// order in which the workers finish doesn't matter.
	jobs := make(chan job)
	results := make(chan result, 9)
	var wg sync.WaitGroup
	for range 4 {
		wg.Add(1)
		go worker(jobs, results, &wg)
	}
	for i := range 9 {
		jobs <- job{i, i + 1}
	}
	close(jobs)
	wg.Wait()
	close(results)
	squares := make([]int, 9)
	for r := range results {
		squares[r.id] = r.square
	}
	fmt.Println("squares:", squares)

	// A pipeline of goroutines: gen -> square -> sum.
	sum := 0
	for v := range square(gen(10)) {
		sum += v
	}
	fmt.Println("sum of squares 1..10:", sum)

	// 8 goroutines update one map; the mutex makes that safe. WaitGroup.Go
	// (Go 1.25) does the Add and the Done.
	var mu sync.Mutex
	counts := map[string]int{}
	var group sync.WaitGroup
	for g := range 8 {
		group.Go(func() {
			for i := range 100 {
				key := "even"
				if (g*100+i)%2 == 1 {
					key = "odd"
				}
				mu.Lock()
				counts[key]++
				mu.Unlock()
			}
		})
	}
	group.Wait()
	fmt.Println("counts:", counts)

	var hits atomic.Int64
	var hitters sync.WaitGroup
	for range 50 {
		hitters.Add(1)
		go func() {
			defer hitters.Done()
			hits.Add(2)
		}()
	}
	hitters.Wait()
	fmt.Println("hits:", hits.Load())

	// select: a ready channel wins over a timer; when nothing is ready, the
	// timer fires.
	ready := make(chan string, 1)
	ready <- "fast"
	select {
	case msg := <-ready:
		fmt.Println("got", msg)
	case <-time.After(time.Second):
		fmt.Println("timeout")
	}
	never := make(chan string)
	select {
	case msg := <-never:
		fmt.Println("got", msg)
	case <-time.After(20 * time.Millisecond):
		fmt.Println("timeout after 20ms")
	}

	// select with default: sends that would block are skipped.
	box := make(chan int, 2)
	sent := 0
	for i := range 5 {
		select {
		case box <- i * 10:
			sent++
		default:
		}
	}
	fmt.Println("sent", sent, "of 5, len", len(box), "cap", cap(box))
	close(box)
	for range 3 {
		v, ok := <-box // after close: the buffered values, then 0 false
		fmt.Println(v, ok)
	}

	// Fan-in: merge two channels until both are closed. A nil channel is
	// never ready, so a closed input is switched off by setting it to nil.
	a, b := gen(3), square(gen(3))
	var merged []int
	for a != nil || b != nil {
		select {
		case v, ok := <-a:
			if !ok {
				a = nil
				continue
			}
			merged = append(merged, v)
		case v, ok := <-b:
			if !ok {
				b = nil
				continue
			}
			merged = append(merged, v)
		}
	}
	slices.Sort(merged)
	fmt.Println("merged:", merged)

	// Unbuffered channels hand a value from one goroutine to another.
	ping, pong := make(chan int), make(chan int)
	go func() {
		for n := range ping {
			pong <- n + 1
		}
		close(pong)
	}()
	n := 0
	for range 3 {
		ping <- n
		n = <-pong
	}
	close(ping)
	_, open := <-pong
	fmt.Println("ping-pong:", n, open)
}
