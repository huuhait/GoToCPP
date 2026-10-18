// phase: 08

// select inside goroutines started by go, closures and methods; results are sorted or collected
// through channels so the output does not depend on scheduling.
package main

import "fmt"

type server struct {
	req  chan int
	quit chan bool
}

func (s *server) loop(resp chan<- string) {
	total := 0
	for {
		select {
		case r := <-s.req:
			total += r
		case <-s.quit:
			resp <- fmt.Sprint("total ", total)
			return
		}
	}
}

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
	s := &server{make(chan int), make(chan bool)}
	resp := make(chan string)
	go s.loop(resp)
	for i := 1; i <= 10; i++ {
		s.req <- i
	}
	s.quit <- true
	fmt.Println(<-resp)

	// Many goroutines select between a value channel and a stop channel.
	vals := make(chan int)
	stop := make(chan struct{})
	out := make(chan int, 100)
	for g := 0; g < 8; g++ {
		go func() {
			n := 0
			for {
				select {
				case v := <-vals:
					n += v
				case <-stop:
					out <- n
					return
				}
			}
		}()
	}
	for i := 1; i <= 100; i++ {
		vals <- i
	}
	close(stop)
	sum := 0
	var parts []int
	for g := 0; g < 8; g++ {
		n := <-out
		sum += n
		parts = append(parts, n)
	}
	sortInts(parts)
	fmt.Println(sum, len(parts))
}

// Output:
// total 55
// 5050 8
