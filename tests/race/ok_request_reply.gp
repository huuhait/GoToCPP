// phase: 08

// A request carries its own reply channel: a client reads what the server wrote.
package main

import "fmt"

type Request struct {
	n     int
	reply chan []int
}

func server(reqs chan Request) {
	for r := range reqs {
		out := make([]int, 0, r.n)
		for i := 0; i < r.n; i++ {
			out = append(out, i*2)
		}
		r.reply <- out
	}
}

func main() {
	reqs := make(chan Request)
	go server(reqs)
	done := make(chan int)
	for c := 1; c <= 4; c++ {
		go func() {
			r := Request{c, make(chan []int)}
			reqs <- r
			out := <-r.reply
			done <- len(out)
		}()
	}
	t := 0
	for c := 0; c < 4; c++ {
		t += <-done
	}
	close(reqs)
	fmt.Println(t)
}

// Output:
// 10
