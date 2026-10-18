// phase: 08

// A goroutine blocked in a select on three channels is woken by exactly one of three senders; the
// other two senders stay blocked and their values are not lost: they are received later.
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
	a, b, c := make(chan string), make(chan string), make(chan string)
	ready := make(chan bool)
	got := make(chan string, 3)
	go func() {
		ready <- true
		select {
		case v := <-a:
			got <- v
		case v := <-b:
			got <- v
		case v := <-c:
			got <- v
		}
		ready <- true
	}()
	<-ready // the goroutine is about to select
	sent := make(chan string, 3)
	go func() { a <- "a"; sent <- "a" }()
	go func() { b <- "b"; sent <- "b" }()
	go func() { c <- "c"; sent <- "c" }()
	<-ready // one of them was received
	first := <-got
	// Exactly one sender has finished (the one whose value the select took).
	one := <-sent
	fmt.Println(first == one)
	// The others are still blocked: receive them, any order, and sort.
	var rest []string
	for i := 0; i < 2; i++ {
		select {
		case v := <-a:
			rest = append(rest, v)
		case v := <-b:
			rest = append(rest, v)
		case v := <-c:
			rest = append(rest, v)
		}
	}
	sortStrings(rest)
	all := append(rest, first)
	sortStrings(all)
	fmt.Println(all, len(rest))
	<-sent
	<-sent
	fmt.Println(len(sent))
}

// Output:
// true
// [a b c] 2
// 0
