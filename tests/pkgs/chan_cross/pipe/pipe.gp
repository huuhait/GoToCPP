package pipe

import "fmt"

// Pipe is a named channel type with methods, exported to other packages.
type Pipe chan int

func (p Pipe) Send(v int) { p <- v }
func (p Pipe) Recv() int  { return <-p }

// Ticket travels through channels of other packages.
type Ticket struct {
	ID   int
	Name string
}

type Namer interface{ Label() string }

func (t Ticket) Label() string { return fmt.Sprint("ticket#", t.ID, " ", t.Name) }

var Default = make(chan string, 4)

func Gen(n int) <-chan int {
	c := make(chan int)
	go func() {
		for i := 1; i <= n; i++ {
			c <- i
		}
		close(c)
	}()
	return c
}

func Worker(in <-chan int, out chan<- Ticket, name string) {
	for {
		v, ok := <-in
		if !ok {
			close(out)
			return
		}
		out <- Ticket{v * 10, name}
	}
}

func Kind(x any) string {
	switch x.(type) {
	case Pipe:
		return "pipe.Pipe"
	case chan int:
		return "chan int"
	case <-chan int:
		return "<-chan int"
	case chan<- Ticket:
		return "chan<- Ticket"
	case chan Ticket:
		return "chan Ticket"
	}
	return "other"
}

type Hub struct {
	In  <-chan Ticket
	Out chan<- string
}

func (h Hub) Run(n int) {
	for i := 0; i < n; i++ {
		t := <-h.In
		h.Out <- t.Label()
	}
}
