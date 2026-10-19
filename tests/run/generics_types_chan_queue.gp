// phase: 08

// A generic type over a channel, and generic types holding channels, used by goroutines.
package main

import "fmt"

type Queue[T any] chan T

func (q Queue[T]) Put(v T) { q <- v }
func (q Queue[T]) Get() T  { return <-q }

type Pipe[T any] struct {
	in  chan T
	out chan T
}

func NewPipe[T any](f func(T) T) *Pipe[T] {
	p := &Pipe[T]{make(chan T), make(chan T)}
	go func() {
		for {
			v, ok := <-p.in
			if !ok {
				close(p.out)
				return
			}
			p.out <- f(v)
		}
	}()
	return p
}

func (p *Pipe[T]) Do(v T) T {
	p.in <- v
	return <-p.out
}

func (p *Pipe[T]) Close() { close(p.in) }

func main() {
	q := make(Queue[string], 2)
	q.Put("a")
	q.Put("b")
	fmt.Println(q.Get(), q.Get(), len(q), cap(q))
	p := NewPipe(func(i int) int { return i * i })
	fmt.Println(p.Do(3), p.Do(4))
	p.Close()
	_, ok := <-p.out
	fmt.Println(ok)
	s := NewPipe(func(s string) string { return s + s })
	fmt.Println(s.Do("ab"))
	s.Close()
	fmt.Printf("%T %T\n", q, s)
}

// Output:
// a b 0 2
// 9 16
// false
// abab
// main.Queue[string] *main.Pipe[string]
