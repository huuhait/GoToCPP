// phase: 08

// Each goroutine fills a struct it owns and sends the pointer.
package main

import "fmt"

type Stat struct {
	id    int
	count int
	names []string
}

func main() {
	ch := make(chan *Stat)
	for g := 0; g < 5; g++ {
		go func() {
			s := &Stat{id: g}
			for i := 0; i < g; i++ {
				s.count++
				s.names = append(s.names, fmt.Sprint(g, i))
			}
			ch <- s
		}()
	}
	n := 0
	for g := 0; g < 5; g++ {
		s := <-ch
		n += s.count + len(s.names)
	}
	fmt.Println(n)
}

// Output:
// 20
