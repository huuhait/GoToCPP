// phase: 08

// Channels of generic types and channels with type-parameter element types: directional channel
// conversions in generic code, select over instance channels, a generic merge/fan-out pipeline,
// nil channels in select, closing, and sending struct instances by value.
package main

import (
	"fmt"
	"slices"
	"sync"
)

type Msg[T any] struct {
	ID   int
	Body T
}

type Pair[A, B any] struct {
	A A
	B B
}

func Gen[T any](vals ...T) <-chan Msg[T] {
	out := make(chan Msg[T])
	go func() {
		defer close(out)
		for i, v := range vals {
			out <- Msg[T]{i, v}
		}
	}()
	return out
}

func Map[A, B any](in <-chan Msg[A], f func(A) B) <-chan Msg[B] {
	out := make(chan Msg[B], 2)
	go func() {
		defer close(out)
		for m := range in {
			out <- Msg[B]{m.ID, f(m.Body)}
		}
	}()
	return out
}

func Merge[T any](cs ...<-chan T) <-chan T {
	out := make(chan T)
	var wg sync.WaitGroup
	for _, c := range cs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for v := range c {
				out <- v
			}
		}()
	}
	go func() { wg.Wait(); close(out) }()
	return out
}

func Zip[A, B any](as <-chan A, bs <-chan B) <-chan Pair[A, B] {
	out := make(chan Pair[A, B])
	go func() {
		defer close(out)
		for {
			a, ok1 := <-as
			b, ok2 := <-bs
			if !ok1 || !ok2 {
				return
			}
			out <- Pair[A, B]{a, b}
		}
	}()
	return out
}

func First[T any](a, b <-chan T) (T, string) {
	select {
	case v, ok := <-a:
		if ok {
			return v, "a"
		}
		return v, "a closed"
	case v, ok := <-b:
		if ok {
			return v, "b"
		}
		return v, "b closed"
	}
}

func Drain[T any](c <-chan T) (out []T) {
	for v := range c {
		out = append(out, v)
	}
	return
}

func ToSend[T any](c chan T) chan<- T { return c }
func ToRecv[T any](c chan T) <-chan T { return c }

func main() {
	sq := Map(Gen(1, 2, 3, 4), func(i int) int8 { return int8(i * 50) })
	fmt.Println(Drain(sq))
	words := Map(Gen("a", "bb", "ccc"), func(s string) Pair[string, int] { return Pair[string, int]{s, len(s)} })
	for m := range words {
		fmt.Printf("%v %T\n", m, m)
	}
	merged := Drain(Merge(Gen(1, 2, 3), Gen(10, 20), Gen[int]()))
	ids := []string{}
	for _, m := range merged {
		ids = append(ids, fmt.Sprint(m.ID, ":", m.Body))
	}
	slices.Sort(ids)
	fmt.Println(ids)
	for p := range Zip(ToRecv(func() chan string {
		c := make(chan string, 3)
		c <- "x"
		c <- "y"
		c <- "z"
		close(c)
		return c
	}()), Gen(1.5, 2.5)) {
		fmt.Printf("%v|", p)
	}
	fmt.Println()
	a, b := make(chan Msg[uint8], 1), make(chan Msg[uint8], 1)
	b <- Msg[uint8]{9, 255}
	fmt.Println(First(a, b))
	close(a)
	fmt.Println(First(a, nil))
	var nilc chan Pair[int, int]
	select {
	case v := <-nilc:
		fmt.Println("never", v)
	default:
		fmt.Println("nil channel blocks")
	}
	c := make(chan [2]Msg[string], 1)
	ToSend(c) <- [2]Msg[string]{{1, "p"}, {2, "q"}}
	fmt.Println(<-ToRecv(c), len(c), cap(c))
	fmt.Printf("%T %T %T\n", ToSend(c), ToRecv(c), c)
}

// Output:
// [{0 50} {1 100} {2 -106} {3 -56}]
// {0 {a 1}} main.Msg[main.Pair[string,int]]
// {1 {bb 2}} main.Msg[main.Pair[string,int]]
// {2 {ccc 3}} main.Msg[main.Pair[string,int]]
// [0:1 0:10 1:2 1:20 2:3]
// {x {0 1.5}}|{y {1 2.5}}|
// {9 255} b
// {0 0} a closed
// nil channel blocks
// [{1 p} {2 q}] 0 1
// chan<- [2]main.Msg[string] <-chan [2]main.Msg[string] chan [2]main.Msg[string]
