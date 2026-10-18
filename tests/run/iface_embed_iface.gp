// phase: 05
// ExitCode: 2
// Stderr: ^panic: runtime error: invalid memory address or nil pointer dereference\n\[signal SIGSEGV: segmentation violation code=0x1 addr=0x[0-9a-f]+ pc=0x[0-9a-f]+\]\n\ngoroutine 1 \[running\]:\n

// An interface embedded in a struct: its methods are promoted, so the struct satisfies the
// interface; the outer type can override a method, and calls through a nil embedded interface
// panic like any call of a nil interface.
package main

import "fmt"

type Speaker interface {
	Speak() string
	Name() string
}

type Dog struct{ name string }

func (d Dog) Speak() string { return "woof" }
func (d Dog) Name() string  { return d.name }

type Loud struct {
	Speaker // the embedded interface
	times   int
}

func (l Loud) Speak() string { // overrides the promoted method
	s := l.Speaker.Speak()
	for i := 1; i < l.times; i++ {
		s += " " + l.Speaker.Speak()
	}
	return s
}

type Named struct {
	Speaker
}

type Wrapper struct {
	Named
	extra int
}

func describe(s Speaker) string { return s.Name() + ": " + s.Speak() }

func main() {
	d := Dog{"rex"}
	l := Loud{Speaker: d, times: 3}
	fmt.Println(describe(d))
	fmt.Println(describe(l))
	fmt.Println(l.Name(), l.Speak(), l.Speaker.Speak())
	n := Named{Speaker: l}
	fmt.Println(describe(n), n.Name())
	w := Wrapper{n, 1}
	fmt.Println(describe(w), w.Name(), w.Speak())
	var sp Speaker = &w // *Wrapper has the promoted methods too
	fmt.Println(sp.Speak())
	// A nil embedded interface in the middle of the chain:
	var empty Named
	fmt.Println(empty.Speaker == nil)
	var sp2 Speaker = empty // allowed: Named has the methods
	fmt.Println(sp2 != nil)
	defer fmt.Println("deferred runs")
	fmt.Println(sp2.Name()) // panics: the embedded interface is nil
}

// Output:
// rex: woof
// rex: woof woof woof
// rex woof woof woof woof
// rex: woof woof woof rex
// rex: woof woof woof rex woof woof woof
// woof woof woof
// true
// true
// deferred runs
