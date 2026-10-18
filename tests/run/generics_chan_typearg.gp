// phase: 08

// Channels of a type parameter's type: make, send, receive, len, cap, close, comma-ok, and directions.
package main

import "fmt"

type Msg struct {
	ID   int
	Text string
}

func Buffered[T any](vs ...T) chan T {
	ch := make(chan T, len(vs))
	for _, v := range vs {
		ch <- v
	}
	return ch
}

func Recv[T any](ch <-chan T) (T, bool) {
	v, ok := <-ch
	return v, ok
}

func Drain[T any](ch chan T) []T {
	close(ch)
	var out []T
	for {
		v, ok := <-ch
		if !ok {
			return out
		}
		out = append(out, v)
	}
}

func Stats[T any](ch chan T) (int, int) { return len(ch), cap(ch) }

func Send[T any](ch chan<- T, vs ...T) {
	for _, v := range vs {
		ch <- v
	}
}

func Zero[T any](ch chan T) T {
	close(ch)
	return <-ch
}

func main() {
	ch := Buffered(1, 2, 3)
	fmt.Println(Stats(ch))
	v, ok := Recv(ch)
	fmt.Println(v, ok, len(ch))
	fmt.Println(Drain(ch))
	sc := Buffered("a", "b")
	fmt.Println(Stats(sc))
	fmt.Println(Drain(sc), len(Drain(Buffered[bool]())))
	mc := make(chan Msg, 2)
	Send(mc, Msg{1, "x"}, Msg{2, "y"})
	fmt.Println(Drain(mc))
	fmt.Println(Zero(make(chan int)), Zero(make(chan string)) == "", Zero(make(chan Msg)), Zero(make(chan *int)) == nil)
	var nilch chan int
	fmt.Println(len(nilch), cap(nilch))
	fmt.Printf("%T %T\n", Buffered(1.5), Buffered[error]())
}

// Output:
// 3 3
// 1 true 2
// [2 3]
// 2 2
// [a b] 0
// [{1 x} {2 y}]
// 0 true {0 } true
// 0 0
// chan float64 chan error
