// phase: 05

// Defer: receivers and arguments are evaluated at the defer statement, results are changed by deferred closures, loops, nil func.
package main

import "fmt"

type T struct{ n int }

func (t T) Val(tag string)  { fmt.Println(tag, "val", t.n) }
func (t *T) Ptr(tag string) { fmt.Println(tag, "ptr", t.n) }

type Doer interface{ Val(string) }

func trace(s string) string { fmt.Println("enter", s); return s }
func un(s string)           { fmt.Println("leave", s) }

func nested() (r int) {
	defer un(trace("nested"))
	defer func() { r *= 2 }()
	defer func() { r += 3 }()
	return 4
}

func loop() {
	for i := 0; i < 3; i++ {
		defer fmt.Println("loop defer", i)
		defer func() { fmt.Println("loop closure", i) }()
	}
}

func recv() {
	t := T{1}
	defer t.Val("A")
	defer t.Ptr("B")
	f := t.Val
	g := t.Ptr
	defer f("C")
	defer g("D")
	var d Doer = t
	defer d.Val("E")
	t.n = 2
	defer t.Val("F")
	h := (*T).Ptr
	defer h(&t, "G")
	t.n = 3
}

func mk() func() {
	fmt.Println("mk called")
	return func() { fmt.Println("returned func runs") }
}

func multi() (a, b int, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("recovered %v", r)
			a, b = -1, -2
		}
	}()
	a, b = 1, 2
	var arr []int
	_ = arr[a+b]
	return 10, 20, nil
}

func deferNilFunc() {
	defer func() { fmt.Println("recovered:", recover()) }()
	var f func()
	defer f()
	fmt.Println("body done")
}

func argsEval() {
	x := 1
	defer fmt.Println("deferred x =", x)
	x = 2
	defer func(v int) { fmt.Println("param v =", v, "x =", x) }(x)
	x = 3
}

func main() {
	fmt.Println(nested())
	loop()
	recv()
	defer mk()()
	fmt.Println(multi())
	deferNilFunc()
	argsEval()
	for i := range 2 {
		defer func() { fmt.Println("main defer", i) }()
	}
	fmt.Println("end of main")
}
