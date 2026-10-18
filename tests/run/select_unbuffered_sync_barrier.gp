// phase: 08

// A barrier made of select: workers wait for the release channel or for a cancel channel; results are
// the same on every run.
package main

import "fmt"

func main() {
	const workers = 10
	release := make(chan struct{})
	cancel := make(chan struct{})
	arrived := make(chan int, workers)
	results := make(chan string, workers)
	for i := 0; i < workers; i++ {
		go func() {
			arrived <- i
			select {
			case <-release:
				results <- "released"
			case <-cancel:
				results <- "cancelled"
			}
		}()
	}
	for i := 0; i < workers; i++ {
		<-arrived
	}
	close(release)
	n := 0
	for i := 0; i < workers; i++ {
		if <-results == "released" {
			n++
		}
	}
	fmt.Println(n)

	// cancel wins when it is closed first; closing release afterwards changes nothing
	release2 := make(chan struct{})
	cancel2 := make(chan struct{})
	res := make(chan string, 1)
	ready := make(chan bool)
	go func() {
		ready <- true
		select {
		case <-release2:
			res <- "released"
		case <-cancel2:
			res <- "cancelled"
		}
	}()
	<-ready
	close(cancel2)
	fmt.Println(<-res)
	close(release2)
}

// Output:
// 10
// cancelled
