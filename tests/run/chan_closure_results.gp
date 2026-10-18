// phase: 08

// Goroutines that each compute a slot of a result and report on a channel of structs; the results are
// put in order by index.
package main

import "fmt"

type Result struct {
	Index int
	Name  string
	Sq    [2]int
}

func main() {
	names := []string{"alpha", "beta", "gamma", "delta", "epsilon"}
	out := make(chan Result)
	for i, n := range names {
		go func() {
			out <- Result{i, n, [2]int{len(n), len(n) * len(n)}}
		}()
	}
	ordered := make([]Result, len(names))
	for range names {
		r := <-out
		ordered[r.Index] = r
	}
	for _, r := range ordered {
		fmt.Println(r.Index, r.Name, r.Sq)
	}
}

// Output:
// 0 alpha [5 25]
// 1 beta [4 16]
// 2 gamma [5 25]
// 3 delta [5 25]
// 4 epsilon [7 49]
