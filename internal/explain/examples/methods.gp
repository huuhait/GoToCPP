package main

import "fmt"

type Acc struct{ total int }

func (a Acc) Get() int   { return a.total }
func (a *Acc) Add(n int) { a.total += n }

func main() {
	a := &Acc{}
	a.Add(2)
	var b Acc
	b.Add(3)
	fmt.Println(a.Get(), b.Get())
}
