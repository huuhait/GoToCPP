// phase: 08

// A value receiver of a go statement is copied by the go statement; a pointer receiver shares.
package main

import "fmt"

type Box struct {
	items []int
	n     int
}

func (b Box) Report(out chan<- string, gate <-chan bool) { <-gate; out <- fmt.Sprint("value: n=", b.n, " items=", b.items) }
func (b *Box) ReportPtr(out chan<- string) { out <- fmt.Sprint("pointer: n=", b.n, " items=", b.items) }

func main() {
	out := make(chan string)
	ready := make(chan bool)
	b := Box{[]int{1, 2}, 1}
	gate := make(chan bool)
	go b.Report(out, gate)
	b.n = 2
	b.items[0] = 100 // the slice is shared by the copy
	gate <- true
	fmt.Println(<-out)

	bp := &b
	go func() {
		<-ready
		bp.ReportPtr(out)
	}()
	b.n = 3
	ready <- true
	fmt.Println(<-out)
}

// Output:
// value: n=1 items=[100 2]
// pointer: n=3 items=[100 2]
