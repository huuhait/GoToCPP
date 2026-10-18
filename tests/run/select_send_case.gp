// phase: 08

// Send cases: chosen when the channel has room or a receiver waits; the value is evaluated
// whether or not the case is chosen.
package main

import "fmt"

var log []string

func val(name string, v int) int {
	log = append(log, name)
	return v
}

func main() {
	full := make(chan int, 1)
	full <- 0
	room := make(chan int, 2)
	select {
	case full <- val("full", 1):
		fmt.Println("full?!")
	case room <- val("room", 2):
		fmt.Println("room")
	}
	got := <-room
	fmt.Println(log, len(full), len(room), got)

	// Unbuffered: a goroutine receives.
	u := make(chan int)
	res := make(chan int)
	go func() { res <- <-u * 2 }()
	select {
	case u <- val("u", 21):
		fmt.Println("sent")
	}
	fmt.Println(<-res, log)
}

// Output:
// room
// [full room] 1 0 2
// sent
// 42 [full room u]
