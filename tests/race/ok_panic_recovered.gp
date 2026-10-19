// phase: 08

// A goroutine panics and recovers, and reports through a channel.
package main

import "fmt"

var log []string

func work(id int, done chan bool) {
	defer func() {
		if r := recover(); r != nil {
			log = append(log, fmt.Sprint("recovered ", r))
		}
		done <- true
	}()
	if id == 1 {
		panic("boom")
	}
}

func main() {
	done := make(chan bool)
	go work(1, done)
	<-done
	go work(2, done)
	<-done
	fmt.Println(log)
}

// Output:
// [recovered boom]
