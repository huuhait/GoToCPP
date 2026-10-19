// phase: 08

// An interface value is built by one goroutine and used by another after a receive.
package main

import "fmt"

type Shape interface{ Area() int }
type Rect struct{ w, h int }
type Square struct{ s int }

func (r *Rect) Area() int   { return r.w * r.h }
func (s *Square) Area() int { return s.s * s.s }

func main() {
	ch := make(chan Shape)
	go func() {
		ch <- &Rect{2, 3}
		ch <- &Square{4}
		close(ch)
	}()
	t := 0
	for s := range ch {
		t += s.Area()
	}
	fmt.Println(t)
}

// Output:
// 22
