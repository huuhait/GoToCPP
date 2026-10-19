package main

type T struct{ n int }

func (t *T) Bump() { t.n++ }

func work(n int, out chan int) { out <- n }

func main() {
	out := make(chan int, 4)
	t := &T{}
	go work(3, out)
	go t.Bump()
	go func() { out <- 1 }()
	f := func() {}
	go f()
	<-out
	<-out
}
