// phase: 08

// A select receive orders the sender's writes before it with what the receiver does next.
package main

import "fmt"

type result struct {
	id    int
	value []int
}

func main() {
	a := make(chan *result)
	b := make(chan *result)
	go func() { a <- &result{1, []int{1}} }()
	go func() { b <- &result{2, []int{2, 2}} }()
	n := 0
	for i := 0; i < 2; i++ {
		select {
		case r := <-a:
			r.value = append(r.value, 10)
			n += len(r.value)
		case r := <-b:
			r.value = append(r.value, 20)
			n += len(r.value)
		}
	}
	fmt.Println(n)
}

// Output:
// 5
