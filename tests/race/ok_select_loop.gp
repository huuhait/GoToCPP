// phase: 08

// A select loop forwards between two channels and quits on a done channel.
package main

import "fmt"

type Event struct{ n int }

func main() {
	in := make(chan *Event)
	out := make(chan *Event)
	quit := make(chan struct{})
	finished := make(chan bool)
	go func() {
		for {
			select {
			case e := <-in:
				e.n *= 2
				out <- e
			case <-quit:
				finished <- true
				return
			}
		}
	}()
	t := 0
	for i := 1; i <= 5; i++ {
		in <- &Event{i}
		e := <-out
		t += e.n
	}
	close(quit)
	<-finished
	fmt.Println(t)
}

// Output:
// 30
