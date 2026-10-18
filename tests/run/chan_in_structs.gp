// phase: 08

// Channels as fields, in arrays, slices and maps; the zero value of each is a nil channel.
package main

import "fmt"

type Bus struct {
	In   chan int
	Out  <-chan string
	Done chan struct{}
}

func newBus() Bus {
	out := make(chan string, 2)
	out <- "hello"
	out <- "world"
	return Bus{In: make(chan int, 1), Out: out, Done: make(chan struct{})}
}

func main() {
	var zero Bus
	fmt.Println(zero.In == nil, zero.Out == nil, zero.Done == nil, len(zero.In), cap(zero.Out))

	b := newBus()
	b.In <- 5
	fmt.Println(<-b.In, <-b.Out, <-b.Out)
	close(b.Done)
	_, ok := <-b.Done
	fmt.Println(ok)

	var chans [3]chan int
	for i := range chans {
		chans[i] = make(chan int, 1)
		chans[i] <- i * 2
	}
	for _, c := range chans {
		fmt.Print(<-c, ",")
	}
	fmt.Println()

	slice := make([]chan string, 0)
	for _, w := range []string{"x", "y"} {
		c := make(chan string, 1)
		c <- w
		slice = append(slice, c)
	}
	fmt.Println(<-slice[1], <-slice[0], len(slice))

	byName := map[string]chan int{}
	byName["a"] = make(chan int, 1)
	byName["a"] <- 1
	fmt.Println(<-byName["a"], byName["missing"] == nil)

	copyOf := b
	copyOf.In <- 77 // the copy shares the channel
	fmt.Println(<-b.In, copyOf.In == b.In)
	p := &b
	p.In <- 78
	fmt.Println(<-b.In)
}

// Output:
// true true true 0 0
// 5 hello world
// false
// 0,2,4,
// y x 2
// 1 true
// 77 true
// 78
