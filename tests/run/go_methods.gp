// phase: 08

// go on methods: the receiver is evaluated by the go statement (a value receiver is copied), also
// for method values, method expressions and interface methods.
package main

import "fmt"

type Acc struct {
	name string
	n    int
}

func (a Acc) Report(out chan<- string, suffix string) { out <- fmt.Sprint(a.name, a.n, suffix) }
func (a *Acc) Add(k int, done chan<- bool)           { a.n += k; done <- true }

type Reporter interface {
	Report(out chan<- string, suffix string)
}

func main() {
	out := make(chan string)
	done := make(chan bool)
	a := Acc{"acc", 1}

	go a.Report(out, "!") // copies a now
	a.n = 100
	fmt.Println(<-out)

	for i := 1; i <= 3; i++ {
		go a.Add(i, done) // (&a).Add: pointer receiver
		<-done
	}
	fmt.Println(a.n)

	f := a.Report // method value: copies a here
	a.n = -1
	go f(out, "?")
	fmt.Println(<-out)

	g := (*Acc).Add // method expression
	go g(&a, 10, done)
	<-done
	fmt.Println(a.n)

	var r Reporter = Acc{"iface", 7}
	go r.Report(out, "#")
	r = Acc{"other", 8} // the interface value was evaluated already
	fmt.Println(<-out)
	h := Acc.Report
	go h(Acc{"expr", 9}, out, "$")
	fmt.Println(<-out)
}

// Output:
// acc1!
// 106
// acc106?
// 9
// iface7#
// expr9$
