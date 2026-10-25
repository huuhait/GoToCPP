// Goroutines and channels: a buffered job channel consumed by two workers with a done channel,
// results read in a fixed order, an unbuffered handoff goroutine, select with a default and
// with two cases, and a receive from a closed channel. The output is deterministic because
// every result is read in a fixed order and the workers only add.
package main

func worker(id int, jobs <-chan int, results chan<- int, done chan<- bool) {
	sum := 0
	for j := range jobs {
		sum += j * id
	}
	results <- sum
	done <- true
}

func main() {
	jobs1 := make(chan int, 3)
	jobs2 := make(chan int, 3)
	results := make(chan int, 2)
	done := make(chan bool, 2)
	go worker(1, jobs1, results, done)
	go worker(2, jobs2, results, done)
	for i := 1; i <= 3; i++ {
		jobs1 <- i
		jobs2 <- i
	}
	close(jobs1)
	close(jobs2)
	<-done
	<-done
	println(<-results + <-results)
	ch := make(chan int, 1)
	ch <- 1
	select {
	case v := <-ch:
		println("got", v)
	default:
		println("empty")
	}
	select {
	case ch <- 2:
		println("sent")
	default:
		println("full")
	}
	select {
	case v := <-ch:
		println("got", v)
	case ch <- 2:
		println("sent")
	}
	quit := make(chan int)
	go func() { quit <- 42 }()
	println(<-quit)
	closed := make(chan int)
	close(closed)
	v, ok := <-closed
	println(v, ok)
}
