// phase: 08

// The results of Timer.Stop and Timer.Reset in every state of a Go 1.23 timer, and what a receive
// sees afterwards.
package main

import (
	"fmt"
	"time"
)

const long = time.Hour

func recvNow(t *time.Timer) string {
	select {
	case <-t.C:
		return "value"
	default:
		return "empty"
	}
}

func main() {
	// pending
	t := time.NewTimer(long)
	fmt.Println("pending: Stop", t.Stop())
	fmt.Println("stopped: Stop", t.Stop())
	fmt.Println("stopped: Reset", t.Reset(long))
	fmt.Println("pending: Reset", t.Reset(long))
	t.Stop()

	// fired but not received
	t = time.NewTimer(time.Millisecond)
	time.Sleep(30 * time.Millisecond)
	fmt.Println("fired unread: Stop", t.Stop())
	fmt.Println("after that:", recvNow(t))

	t = time.NewTimer(time.Millisecond)
	time.Sleep(30 * time.Millisecond)
	fmt.Println("fired unread: Reset", t.Reset(long))
	fmt.Println("after that:", recvNow(t))
	t.Stop()

	// fired and received
	t = time.NewTimer(time.Millisecond)
	<-t.C
	fmt.Println("fired read: Stop", t.Stop())
	t = time.NewTimer(time.Millisecond)
	<-t.C
	fmt.Println("fired read: Reset", t.Reset(time.Millisecond))
	<-t.C
	fmt.Println("reset timer fired again", recvNow(t))

	// Reset to zero fires at once
	t = time.NewTimer(long)
	t.Reset(0)
	<-t.C
	fmt.Println("reset to 0 fired")

	// Stop twice, Reset twice
	t = time.NewTimer(long)
	fmt.Println(t.Reset(long), t.Reset(time.Millisecond))
	<-t.C
	fmt.Println(t.Stop(), t.Stop())

	// AfterFunc
	ran := make(chan int, 4)
	af := time.AfterFunc(long, func() { ran <- 1 })
	fmt.Println("AfterFunc pending: Stop", af.Stop())
	fmt.Println("AfterFunc stopped: Reset", af.Reset(time.Millisecond))
	<-ran
	fmt.Println("AfterFunc fired: Stop", af.Stop())
	fmt.Println("AfterFunc fired: Reset", af.Reset(time.Millisecond))
	<-ran
	fmt.Println(len(ran))
}

// Output:
// pending: Stop true
// stopped: Stop false
// stopped: Reset false
// pending: Reset true
// fired unread: Stop true
// after that: empty
// fired unread: Reset true
// after that: empty
// fired read: Stop false
// fired read: Reset false
// reset timer fired again empty
// reset to 0 fired
// true true
// false false
// AfterFunc pending: Stop true
// AfterFunc stopped: Reset false
// AfterFunc fired: Stop false
// AfterFunc fired: Reset false
// 0
