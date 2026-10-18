// phase: 08

// Priority with nested selects: try the high channel first without blocking, then wait on both. The
// order of the result is fixed by the structure, not by the random choice.
package main

import "fmt"

func next(high, low chan string) string {
	select {
	case v := <-high:
		return v
	default:
	}
	select {
	case v := <-high:
		return v
	case v := <-low:
		return v
	}
}

func main() {
	high := make(chan string, 10)
	low := make(chan string, 10)
	for i := 0; i < 3; i++ {
		low <- fmt.Sprint("low", i)
		high <- fmt.Sprint("high", i)
	}
	for i := 0; i < 6; i++ {
		fmt.Println(next(high, low))
	}
}

// Output:
// high0
// high1
// high2
// low0
// low1
// low2
