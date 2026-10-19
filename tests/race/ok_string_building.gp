// phase: 08

// Strings built by goroutines and handed over by channel: no sharing is left.
package main

import "fmt"

func main() {
	words := []string{"a", "b", "c", "d"}
	out := make(chan string)
	for _, w := range words {
		go func() {
			s := ""
			for i := 0; i < 3; i++ {
				s += w
			}
			out <- s
		}()
	}
	total := 0
	for range words {
		total += len(<-out)
	}
	fmt.Println(total)
}

// Output:
// 12
