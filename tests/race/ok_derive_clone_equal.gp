// phase: 08

// #Derive(Clone, Equal) on a value that goroutines share (step 10.10): reading it to clone it or to
// compare it is not a write, and each goroutine changes only its own copy, so the race detector
// stays silent.
package main

import (
	"fmt"
	"sync"
)

#Derive(Clone, Equal)
type Config struct {
	Name  string
	Ports []int
	Env   map[string]string
	Next  *Config
}

func main() {
	shared := Config{Name: "base", Ports: []int{1, 2, 3}, Env: map[string]string{"k": "v"}, Next: &Config{Name: "next", Ports: []int{9}}}
	var wg sync.WaitGroup
	var mu sync.Mutex
	same, total := 0, 0
	for i := range 8 {
		wg.Go(func() {
			for range 50 {
				c := shared.Clone()
				c.Ports[0] = i
				c.Env["own"] = "x"
				c.Next.Ports[0]++
				ok := shared.Equal(shared) && !shared.Equal(c)
				mu.Lock()
				total++
				if ok {
					same++
				}
				mu.Unlock()
			}
		})
	}
	wg.Wait()
	fmt.Println(same, total, shared.Ports, shared.Env, shared.Next.Ports)
}

// Output:
// 400 400 [1 2 3] map[k:v] [9]
