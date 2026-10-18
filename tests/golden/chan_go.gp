// phase: 08

// The C++ of goroutines and channels (steps 8.2 to 8.4): go statements of a function, a func
// literal, a method and a builtin, make, send, receive (also with ok), close, len, cap, a
// directional channel and a type switch on channel types.
package main

type T struct{ n int }

func (t T) Show(out chan<- int) { out <- t.n }

func worker(in <-chan int, out chan<- int) {
	for {
		v, ok := <-in
		if !ok {
			close(out)
			return
		}
		out <- v * 2
	}
}

func kind(x any) string {
	switch x.(type) {
	case chan int:
		return "bidirectional"
	case <-chan int:
		return "receive-only"
	}
	return "other"
}

func main() {
	in, out := make(chan int, 4), make(chan int)
	go worker(in, out)
	go func() { in <- 1; close(in) }()
	t := T{3}
	go t.Show(out)
	go close(make(chan bool))
	println(<-out, <-out, len(in), cap(in), kind(in), kind((<-chan int)(in)))
}
