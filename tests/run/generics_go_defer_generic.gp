// phase: 08

// go and defer statements that call generic functions: explicit arguments, inferred ones, and instances as func values.
package main

import "fmt"

func Send[T any](ch chan<- T, v T) { ch <- v }

func Log[T any](tag string, v T) { fmt.Println(tag, v) }

func Worker[T any](id int, in <-chan T, out chan<- string) {
	for {
		v, ok := <-in
		if !ok {
			out <- fmt.Sprint("worker ", id, " done")
			return
		}
		out <- fmt.Sprint("worker ", id, " got ", v)
	}
}

func Spawn[T any](f func(T), v T, done chan bool) {
	go func() {
		f(v)
		done <- true
	}()
}

func main() {
	ch := make(chan int, 3)
	go Send(ch, 1)
	go Send[int](ch, 2)
	a, b := <-ch, <-ch
	fmt.Println(a+b, a != b)
	sc := make(chan string, 1)
	go Send(sc, "s")
	fmt.Println(<-sc)
	defer Log("deferred", 42)
	defer Log[string]("deferred", "str")
	in, out := make(chan float64), make(chan string)
	go Worker(7, in, out)
	in <- 1.5
	fmt.Println(<-out)
	close(in)
	fmt.Println(<-out)
	done := make(chan bool)
	Spawn(func(s string) { fmt.Println("spawned", s) }, "x", done)
	<-done
	logger := func(i int) { Log("logger", i) }
	Spawn(logger, 5, done)
	<-done
}

// Output:
// 3 true
// s
// worker 7 got 1.5
// worker 7 done
// spawned x
// logger 5
// deferred str
// deferred 42
