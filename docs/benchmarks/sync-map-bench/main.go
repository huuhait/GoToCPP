package main

import (
	"fmt"
	"os"
	"sync"
	"time"
)

const keys = 1000

func run(goroutines, iters int, body func(g, i int)) float64 {
	var wg sync.WaitGroup
	start := time.Now()
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < iters; i++ {
				body(g, i)
			}
		}()
	}
	wg.Wait()
	return float64(time.Since(start).Nanoseconds()) / float64(goroutines*iters)
}

func main() {
	mode := os.Args[1]
	var m sync.Map
	for i := 0; i < keys; i++ {
		m.Store(i, i)
	}
	var ns float64
	switch mode {
	case "load1":
		ns = run(1, 2000000, func(g, i int) { m.Load(i % keys) })
	case "load8":
		ns = run(8, 100000, func(g, i int) { m.Load((i + g) % keys) })
	case "load64":
		ns = run(64, 5000, func(g, i int) { m.Load((i + g) % keys) })
	case "load64w1":
		stop := make(chan bool)
		go func() {
			for i := 0; ; i++ {
				select {
				case <-stop:
					return
				default:
				}
				m.Store(i%keys, i)
			}
		}()
		ns = run(64, 5000, func(g, i int) { m.Load((i + g) % keys) })
		close(stop)
	case "storeexisting":
		ns = run(1, 1000000, func(g, i int) { m.Store(i%keys, i) })
	case "storenewdelete":
		ns = run(1, 500000, func(g, i int) {
			m.Store(keys+i%keys, i)
			m.Delete(keys + i%keys)
		}) / 2
	case "loadorstore":
		ns = run(1, 1000000, func(g, i int) { m.LoadOrStore(i%keys, i) })
	case "cas":
		ns = run(1, 1000000, func(g, i int) { m.CompareAndSwap(i%keys, i%keys, i%keys) })
	case "rangeall":
		ns = run(1, 2000, func(g, i int) {
			m.Range(func(k, v any) bool { return true })
		})
	case "rangestop3":
		ns = run(1, 20000, func(g, i int) {
			n := 0
			m.Range(func(k, v any) bool { n++; return n < 3 })
		})
	case "rangestop3churn":
		ns = run(1, 20000, func(g, i int) {
			m.Store(keys+i, i) // a new key: the set of keys changes before each Range
			n := 0
			m.Range(func(k, v any) bool { n++; return n < 3 })
		})
	}
	fmt.Printf("%s %.1f\n", mode, ns)
}
