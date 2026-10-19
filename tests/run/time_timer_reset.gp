// phase: 08

// Timer.Reset: new deadlines reorder timers; Reset of a stopped timer; Reset returns whether the timer was active.
package main

import (
	"fmt"
	"time"
)

func main() {
	a := time.NewTimer(time.Hour)
	b := time.NewTimer(40 * time.Millisecond)
	// a was set to expire after b, now it expires first
	fmt.Println(a.Reset(10 * time.Millisecond))
	ta := <-a.C
	tb := <-b.C
	fmt.Println(ta.Before(tb))
	// a stopped timer can be reset and fires once
	a.Stop()
	fmt.Println(a.Reset(5 * time.Millisecond))
	<-a.C
	fmt.Println(a.Reset(time.Hour), a.Reset(time.Hour), a.Stop())
	// many resets of one timer: it fires only for the last
	t := time.NewTimer(time.Hour)
	for i := 1; i <= 50; i++ {
		t.Reset(time.Duration(i) * time.Hour)
	}
	t.Reset(15 * time.Millisecond)
	s := time.Now()
	<-t.C
	fmt.Println(time.Since(s) >= 10*time.Millisecond)
	time.Sleep(30 * time.Millisecond)
	fmt.Println(len(t.C))
	// a timer reset by the goroutine that receives from it: a repeating timer by hand
	n := 0
	t.Reset(time.Millisecond)
	for n < 5 {
		<-t.C
		n++
		t.Reset(time.Millisecond)
	}
	fmt.Println(n, t.Stop())
	// Reset with d <= 0 fires at once
	t.Reset(0)
	<-t.C
	t.Reset(-time.Hour)
	<-t.C
	fmt.Println("immediate", t.Stop())
	// reorder: timers 1..5 with deadlines reversed
	ts := make([]*time.Timer, 5)
	for i := range ts {
		ts[i] = time.NewTimer(time.Hour)
	}
	// (the order is the order of the times that the timers sent, which are their deadlines: the order in
	// which the goroutines run after the wake-up is up to the machine's load)
	type got struct {
		i int
		v time.Time
	}
	res := make(chan got, 5)
	for i := range ts {
		go func() {
			v := <-ts[i].C
			res <- got{i, v}
		}()
	}
	for i := range ts {
		ts[i].Reset(time.Duration(5-i) * 40 * time.Millisecond)
	}
	var all []got
	for range ts {
		all = append(all, <-res)
	}
	for i := 1; i < len(all); i++ {
		for j := i; j > 0 && all[j].v.Before(all[j-1].v); j-- {
			all[j], all[j-1] = all[j-1], all[j]
		}
	}
	for _, g := range all {
		fmt.Print(g.i, ";")
	}
	fmt.Println()
}

// Output:
// true
// true
// false
// false true true
// true
// 0
// 5 true
// immediate false
// 4;3;2;1;0;
