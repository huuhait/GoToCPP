// phase: 08

// Channels, and values that hold them, in the gdb printers.
package main

import "fmt"

type Msg struct {
	ID   int
	Text string
}

func main() {
	var nilCh chan int
	ch := make(chan int, 3)
	ch <- 1
	ch <- 2
	unbuf := make(chan string)
	closed := make(chan Msg, 1)
	closed <- Msg{1, "m"}
	close(closed)
	var recvOnly <-chan int = ch
	var sendOnly chan<- int = ch
	var a any = ch
	chs := []chan int{ch, nilCh}
	m := map[string]chan int{"c": ch}
	fmt.Println(nilCh, ch, unbuf, closed, recvOnly, sendOnly, a, chs, m, len(ch), cap(ch)) // @chans
}
