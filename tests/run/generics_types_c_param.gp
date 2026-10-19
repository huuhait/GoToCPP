// phase: 08

// Type parameters of generic types may be named C, as in Go code that composes or makes channels.
package main

import "fmt"

type Box[C any] struct{ v C }

func (b Box[C]) Get() C   { return b.v }
func (b *Box[C]) Set(v C) { b.v = v }

type Pair[A, C any] struct {
	a A
	c C
}

func (p Pair[A, C]) Second() C { return p.c }

type Chans[C ~chan E, E any] struct{ ch C }

func (c Chans[C, E]) Send(v E) { c.ch <- v }
func (c Chans[C, E]) Recv() E  { return <-c.ch }

func main() {
	b := Box[string]{"x"}
	b.Set("y")
	fmt.Println(b.Get(), Pair[int, string]{1, "two"}.Second())
	c := Chans[chan int, int]{make(chan int, 1)}
	c.Send(4)
	fmt.Println(c.Recv())
	fmt.Printf("%T %T\n", b, c)
}

// Output:
// y two
// 4
// main.Box[string] main.Chans[chan int,int]
