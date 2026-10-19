// phase: 05

// Go names in backtraces: functions, methods (value and pointer receivers), func literals,
// nested literals, deferred calls, and stepping with .gp lines.
package main

import "fmt"

type T struct{ n int }

func (t T) Get() int {
	return t.n // @get
}

func (t *T) Inc() {
	t.n++ // @inc
}

func outer() func() int {
	x := 10
	f := func() int {
		g := func() int {
			return x + 1 // @inner
		}
		return g() + 1 // @middle
	}
	return f
}

func deferred() {
	defer func() {
		fmt.Println("deferred") // @defer
	}()
	fmt.Println("body")
}

func fact(n int) int {
	if n <= 1 {
		return 1 // @fact
	}
	return n * fact(n-1)
}

var global = func() int {
	return 7 // @global
}()

func main() {
	t := &T{1}
	t.Inc()
	fmt.Println(t.Get())
	fmt.Println(outer()())
	deferred()
	fmt.Println(fact(3), global)
	for i := 0; i < 2; i++ {
		fmt.Println(i) // @loop
	}
}
