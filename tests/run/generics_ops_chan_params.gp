// phase: 08

// Channels with a type parameter element: directions, close, range, cap, len, nil channels, comma-ok receive.
package main

import "fmt"

type Msg struct {
	ID   int
	Body string
}

func Gen[T any](xs ...T) <-chan T {
	c := make(chan T, len(xs))
	for _, x := range xs {
		c <- x
	}
	close(c)
	return c
}

func Collect[T any](c <-chan T) (out []T) {
	for v := range c {
		out = append(out, v)
	}
	return
}

func Send[T any](c chan<- T, vs ...T) {
	for _, v := range vs {
		c <- v
	}
}

func Recv[T any](c <-chan T) (T, bool) {
	v, ok := <-c
	return v, ok
}

func Pipe[A, B any](in <-chan A, f func(A) B) <-chan B {
	out := make(chan B)
	go func() {
		defer close(out)
		for a := range in {
			out <- f(a)
		}
	}()
	return out
}

func Sizes[T any](c chan T) (int, int) { return len(c), cap(c) }

func Bidi[CH ~chan E, E any](n int) CH { return make(CH, n) }

type Pipeline chan int

func Sum[T ~int | ~float64](c <-chan T) (t T) {
	for v := range c {
		t += v
	}
	return
}

func main() {
	fmt.Println(Collect(Gen(1, 2, 3)), Collect(Gen("a", "b")), Collect(Gen[Msg]()), Collect(Gen(Msg{1, "x"})))
	c := make(chan Msg, 3)
	Send(c, Msg{1, "a"}, Msg{2, "b"})
	fmt.Println(Sizes(c))
	v, ok := Recv(c)
	fmt.Println(v, ok)
	close(c)
	Recv(c)
	v, ok = Recv(c)
	fmt.Println(v, ok)
	sq := Pipe(Gen(1, 2, 3, 4), func(i int) string { return fmt.Sprint(i * i) })
	fmt.Println(Collect(sq))
	p := Bidi[Pipeline](4)
	Send(p, 5, 6, 7)
	close(p)
	fmt.Println(Sum(p), len(Bidi[chan string](0)), cap(Bidi[Pipeline](9)))
	fmt.Println(Sum(Gen(0.5, 0.25)))
	var nilc chan int
	fmt.Println(Sizes(nilc))
	defer func() { fmt.Println("recovered:", recover()) }()
	Send(c, Msg{})
}

// Output:
// [1 2 3] [a b] [] [{1 x}]
// 2 3
// {1 a} true
// {0 } false
// [1 4 9 16]
// 18 0 9
// 0.75
// 0 0
// recovered: send on closed channel
