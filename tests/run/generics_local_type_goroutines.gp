// phase: 09

// A generic type declared inside a generic function is used by goroutines and sent over a channel.
package main

import (
	"fmt"
	"sort"
	"sync"
)

func Gather[T any](xs []T) int {
	type Res[U any] struct {
		in  T
		out U
	}
	var mu sync.Mutex
	var wg sync.WaitGroup
	var all []Res[int]
	for i, x := range xs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			mu.Lock()
			all = append(all, Res[int]{x, i * 10})
			mu.Unlock()
		}()
	}
	wg.Wait()
	sort.Slice(all, func(i, j int) bool { return all[i].out < all[j].out })
	fmt.Println(all)
	ch := make(chan Res[string], 1)
	ch <- Res[string]{xs[0], "sent"}
	fmt.Println(<-ch)
	return len(all)
}

func main() {
	fmt.Println(Gather([]string{"a", "b", "c"}))
	fmt.Println(Gather([]float64{1.5}))
}

// Output:
// [{a 0} {b 10} {c 20}]
// {a sent}
// 3
// [{1.5 0}]
// {1.5 sent}
// 1
