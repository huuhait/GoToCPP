// phase: 08

// The C++ of select and of range over channels (steps 8.5 and 8.6): receive and send cases, the
// comma-ok and assignment forms, a case whose value is dropped, default, break and continue, a
// select without default whose cases all return (a terminating statement), select {}, a channel
// that is a field (held by a copy) and a call (held as it is), and range loops with and without a
// variable, over a directional channel and with the assignment form.
package main

type box struct{ c chan int }

func src() chan int { return make(chan int, 1) }

func pick(a, b chan int) int {
	select {
	case v := <-a:
		return v
	case v := <-b:
		return -v
	}
}

func main() {
	a, b, done := make(chan int, 1), make(chan string), make(chan struct{})
	x := box{a}
	var n int
	var ok bool
	for i := 0; i < 3; i++ {
		select {
		case v := <-a:
			println("a", v)
		case s, ok := <-b:
			println(s, ok)
			if !ok {
				break
			}
			continue
		case n, ok = <-x.c:
			println(n, ok)
		case b <- "x":
		case x.c <- n + 1:
		case <-src():
		case <-done:
			return
		default:
			println("none")
		}
	}
	var in <-chan int = a
	for v := range in {
		println(v)
	}
	for range a {
	}
	for n = range a {
	}
	println(pick(a, a), n, ok)
	select {}
}
