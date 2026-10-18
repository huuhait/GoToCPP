// phase: 08

// A WaitGroup made of a channel: the main goroutine waits for N completions, and collects each
// worker's result in a slot of its own.
package main

import "fmt"

type Group struct {
	done chan struct{}
	n    int
}

func (g *Group) Go(f func()) {
	g.n++
	go func() {
		f()
		g.done <- struct{}{}
	}()
}

func (g *Group) Wait() {
	for ; g.n > 0; g.n-- {
		<-g.done
	}
}

func main() {
	g := &Group{done: make(chan struct{})}
	results := make([]int, 10)
	for i := range results {
		g.Go(func() {
			s := 0
			for k := 0; k <= i*1000; k++ {
				s += k
			}
			results[i] = s
		})
	}
	g.Wait()
	fmt.Println(results)
}

// Output:
// [0 500500 2001000 4501500 8002000 12502500 18003000 24503500 32004000 40504500]
