// phase: 05

// defer with a method value of an interface method (step 5.9): the interface value is bound
// where defer runs, a deferred func value is a copy, and a nil interface panics at the defer
// statement.
package main

import "fmt"

type Logger interface{ Log(msg string) }

type Pfx struct{ p string }

func (l Pfx) Log(m string) { fmt.Println(l.p + m) }

type Rec struct{ lines *[]string }

func (r Rec) Log(m string) { *r.lines = append(*r.lines, m) }

func order() {
	var l Logger = Pfx{"a:"}
	defer l.Log("direct")
	l = Pfx{"b:"}
	f := l.Log
	defer f("var")
	l = Pfx{"c:"}
	defer l.Log("last")
	l = Pfx{"d:"}
	f("immediate")
}

func nilDefer() (res string) {
	defer func() { res = fmt.Sprint("recovered: ", recover()) }()
	var l Logger
	defer l.Log("never")
	fmt.Println("not reached")
	return "normal"
}

func loop() {
	var lines []string
	var l Logger = Rec{&lines}
	for i := 0; i < 3; i++ {
		defer l.Log(fmt.Sprint("i=", i))
		l = Rec{&lines}
	}
	defer func() { fmt.Println(lines) }()
}

func closure() {
	var l Logger = Pfx{"x:"}
	defer func() { l.Log("closure sees the new value") }()
	l = Pfx{"y:"}
}

func main() {
	order()
	fmt.Println(nilDefer())
	loop()
	closure()
}

// Output:
// b:immediate
// c:last
// b:var
// a:direct
// recovered: runtime error: invalid memory address or nil pointer dereference
// []
// y:closure sees the new value
