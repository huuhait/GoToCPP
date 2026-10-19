// phase: 08

// AfterFunc Stop: true if the call prevents the function from running; false after it was started.
package main

import (
	"fmt"
	"time"
)

func main() {
	ran := make(chan string, 10)
	t := time.AfterFunc(20*time.Millisecond, func() { ran <- "ran" })
	fmt.Println(t.Stop(), t.Stop())
	time.Sleep(60 * time.Millisecond)
	fmt.Println(len(ran))
	// after the function started: false
	t = time.AfterFunc(5*time.Millisecond, func() { ran <- "ran" })
	fmt.Println(<-ran, t.Stop())
	// Stop does not wait for the function
	release := make(chan bool)
	started := make(chan bool)
	t = time.AfterFunc(time.Millisecond, func() {
		started <- true
		<-release
		ran <- "finished"
	})
	<-started
	fmt.Println(t.Stop())
	close(release)
	fmt.Println(<-ran)
	// Stop of many timers: none runs
	n := 1000
	var ts []*time.Timer
	for i := 0; i < n; i++ {
		ts = append(ts, time.AfterFunc(time.Hour, func() { ran <- "bad" }))
	}
	stopped := 0
	for _, x := range ts {
		if x.Stop() {
			stopped++
		}
	}
	fmt.Println(stopped, len(ran))
	// Reset of a stopped AfterFunc timer runs it again
	t = time.AfterFunc(time.Hour, func() { ran <- "again" })
	t.Stop()
	fmt.Println(t.Reset(time.Millisecond))
	fmt.Println(<-ran)
	// the zero Timer, and a Timer from a struct literal
	func() {
		defer func() { fmt.Println("recovered:", recover()) }()
		var z time.Timer
		z.Stop()
	}()
}

// Output:
// true false
// 0
// ran false
// false
// finished
// 1000 0
// false
// again
// recovered: time: Stop called on uninitialized Timer
