// phase: 08

// The same channel in several cases: two receives, a send and a receive, a send twice. The channel
// is locked once; a blocked select with the channel in two cases is woken once.
package main

import "fmt"

func sortInts(a []int) {
	for i := 1; i < len(a); i++ {
		for j := i; j > 0 && a[j-1] > a[j]; j-- {
			a[j-1], a[j] = a[j], a[j-1]
		}
	}
}

func sortStrings(a []string) {
	for i := 1; i < len(a); i++ {
		for j := i; j > 0 && a[j-1] > a[j]; j-- {
			a[j-1], a[j] = a[j], a[j-1]
		}
	}
}

func main() {
	c := make(chan int)
	done := make(chan []int)
	go func() {
		var got []int
		for i := 0; i < 4; i++ {
			select {
			case v := <-c:
				got = append(got, v)
			case v := <-c:
				got = append(got, v*10)
			}
		}
		done <- got
	}()
	for i := 1; i <= 4; i++ {
		c <- i
	}
	got := <-done
	sortInts(got)
	for i, v := range got {
		if v >= 10 {
			v /= 10
		}
		got[i] = v
	}
	sortInts(got)
	fmt.Println(got)

	// A buffered channel with room and a value: both a send and a receive are ready.
	b := make(chan int, 2)
	b <- 1
	sends, recvs := 0, 0
	for i := 0; i < 1000; i++ {
		select {
		case b <- 2:
			sends++
			<-b
		case v := <-b:
			recvs++
			b <- v
		}
	}
	fmt.Println(sends+recvs, sends > 100, recvs > 100)

	// Sends twice to an unbuffered channel, one receiver.
	u := make(chan int)
	res := make(chan int)
	go func() { res <- <-u }()
	select {
	case u <- 1:
	case u <- 2:
	}
	r := <-res
	fmt.Println(r == 1 || r == 2)
}

// Output:
// [1 2 3 4]
// 1000 true true
// true
