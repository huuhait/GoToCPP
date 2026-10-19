// phase: 08

// Ten thousand timers: AfterFunc timers with spread deadlines all run; NewTimer timers all stop; channel timers all deliver.
package main

import (
	"fmt"
	"time"
)

func main() {
	const n = 10000
	// AfterFunc: each sends its index
	res := make(chan int, n)
	for i := 0; i < n; i++ {
		time.AfterFunc(time.Duration(i%40)*time.Millisecond, func() { res <- i })
	}
	seen := make([]bool, n)
	sum := 0
	for i := 0; i < n; i++ {
		v := <-res
		if seen[v] {
			fmt.Println("duplicate", v)
		}
		seen[v] = true
		sum += v
	}
	fmt.Println(sum == n*(n-1)/2, len(res))
	// NewTimer with long deadlines, stopped: Stop reports true for each, nothing ever arrives
	ts := make([]*time.Timer, n)
	for i := range ts {
		ts[i] = time.NewTimer(time.Duration(i+1) * time.Hour)
	}
	stopped := 0
	for _, t := range ts {
		if t.Stop() {
			stopped++
		}
	}
	fmt.Println(stopped)
	// NewTimer with short deadlines: every channel delivers
	for i := range ts {
		ts[i] = time.NewTimer(time.Duration(i%30) * time.Millisecond)
	}
	got := 0
	for _, t := range ts {
		<-t.C
		got++
	}
	fmt.Println(got)
	// the same with After, received by one goroutine each
	done := make(chan int, n)
	for i := 0; i < 2000; i++ {
		go func() {
			<-time.After(time.Duration(i%20) * time.Millisecond)
			done <- i
		}()
	}
	total := 0
	for i := 0; i < 2000; i++ {
		total += <-done
	}
	fmt.Println(total == 2000*1999/2)
	// deadlines reversed: arming order is not firing order
	order := make(chan int, 300)
	// (absolute deadlines from one base, 2 ms apart, so the time spent arming does not matter)
	base := time.Now().Add(500 * time.Millisecond)
	for i := 299; i >= 0; i-- {
		time.AfterFunc(time.Until(base.Add(time.Duration(i)*2*time.Millisecond)), func() { order <- i })
	}
	inorder := 0
	prev := -1
	for i := 0; i < 300; i++ {
		v := <-order
		if v > prev {
			inorder++
		}
		prev = v
	}
	fmt.Println(inorder >= 150)
}

// Output:
// true 0
// 10000
// 10000
// true
// true
