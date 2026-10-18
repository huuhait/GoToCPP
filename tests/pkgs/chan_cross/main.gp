// phase: 08

// Channels across packages (Phase 08 with 06): a named channel type exported by a package, channels
// of another package's struct, go statements that call another package's functions and methods, and
// type switches on channel types in the package that declares them and in another.
package main

import (
	"fmt"

	"example.com/chans/pipe"
	"example.com/chans/side"
)

func main() {
	tickets := make(chan pipe.Ticket)
	go pipe.Worker(pipe.Gen(3), tickets, "w")
	for t := range [3]int{} {
		_ = t
		fmt.Println((<-tickets).Label())
	}
	_, ok := <-tickets
	fmt.Println("closed:", !ok)

	p := make(pipe.Pipe, 2)
	go p.Send(41)
	fmt.Println(p.Recv()+1, len(p), cap(p))
	var plain chan int = p
	var ro <-chan int = p
	fmt.Println(pipe.Kind(p), pipe.Kind(plain), pipe.Kind(ro), pipe.Kind(make(chan<- pipe.Ticket)), pipe.Kind(tickets))

	in, out := make(chan pipe.Ticket, 2), make(chan pipe.Namer, 2)
	in <- pipe.Ticket{ID: 1, Name: "a"}
	in <- pipe.Ticket{ID: 2, Name: "b"}
	go side.Relay(in, out, 2)
	fmt.Println((<-out).Label(), (<-out).Label())

	side.Boxes <- p
	got := <-side.Boxes
	fmt.Println(got == p, any(got) == any(p))

	pipe.Default <- "default channel"
	labels := make(chan string)
	feed := make(chan pipe.Ticket, 1)
	feed <- pipe.Ticket{ID: 7, Name: "hub"}
	go pipe.Hub{In: feed, Out: labels}.Run(1)
	fmt.Println(<-pipe.Default, <-labels)
}

// Output:
// ticket#10 w
// ticket#20 w
// ticket#30 w
// closed: true
// 42 0 2
// pipe.Pipe chan int <-chan int chan<- Ticket chan Ticket
// ticket#1 a ticket#2 b
// true true
// default channel ticket#7 hub
