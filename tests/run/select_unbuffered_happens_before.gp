// phase: 08

// A send happens before the matching receive completes (also through select), so data written before
// the send is visible after the receive; and the k-th receive of a buffered channel happens before the
// (k+C)-th send completes.
package main

import "fmt"

type msg struct {
	payload [16]int
	seq     int
}

func main() {
	c := make(chan *msg)
	ack := make(chan bool)
	go func() {
		for i := 0; i < 1000; i++ {
			select {
			case m := <-c:
				sum := 0
				for _, x := range m.payload {
					sum += x
				}
				if sum != m.seq*16 || m.seq != i {
					fmt.Println("bad", i, sum)
				}
				m.seq = -1 // visible to the sender after ack
				select {
				case ack <- true:
				}
			}
		}
	}()
	bad := 0
	for i := 0; i < 1000; i++ {
		m := &msg{seq: i}
		for j := range m.payload {
			m.payload[j] = i
		}
		select {
		case c <- m:
		}
		select {
		case <-ack:
		}
		if m.seq != -1 {
			bad++
		}
	}
	fmt.Println("bad", bad)

	// a buffered channel as a semaphore: at most 3 goroutines in the critical section
	sem := make(chan struct{}, 3)
	done := make(chan bool)
	inside, maxInside := 0, 0
	lock := make(chan bool, 1)
	for i := 0; i < 20; i++ {
		go func() {
			select {
			case sem <- struct{}{}:
			}
			lock <- true
			inside++
			if inside > maxInside {
				maxInside = inside
			}
			<-lock
			for k := 0; k < 100; k++ {
				select {
				default:
				}
			}
			lock <- true
			inside--
			<-lock
			<-sem
			done <- true
		}()
	}
	for i := 0; i < 20; i++ {
		<-done
	}
	fmt.Println(maxInside <= 3, inside)
}

// Output:
// bad 0
// true 0
