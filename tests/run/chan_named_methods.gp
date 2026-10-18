// phase: 08

// A named channel type can have methods; its values are channels (equal when they are the same
// channel, nil when not made).
package main

import "fmt"

type Queue chan int

func (q Queue) Push(v int) { q <- v }
func (q Queue) Pop() int   { return <-q }
func (q Queue) Len() int   { return len(q) }

type Events chan string

func (e Events) Emit(s string) { e <- s }

func (e Events) Drain() []string {
	var out []string
	for len(e) > 0 {
		out = append(out, <-e)
	}
	return out
}

type Holder struct {
	Q    Queue
	Name string
}

func main() {
	q := make(Queue, 4)
	q.Push(1)
	q.Push(2)
	q.Push(3)
	fmt.Println(q.Len(), q.Pop(), q.Len())

	e := make(Events, 8)
	for _, s := range []string{"a", "b", "c"} {
		e.Emit(s)
	}
	fmt.Println(e.Drain(), len(e))

	var h Holder
	fmt.Println(h.Q == nil, h.Q.Len())
	h.Q = q
	go h.Q.Push(10)
	fmt.Println(q.Pop() + q.Pop() + q.Pop())
	var nq Queue
	fmt.Println(nq == nil, nq.Len())
}

// Output:
// 3 1 2
// [a b c] 0
// true 0
// 15
// true 0
