// The plain-Go twin of umacro_refs.gp: the macros are functions that take func values.

package main

import (
	"fmt"
	"sync"
)

func Twice(f func() int) int { return f() + f() }

func Keep(f func() int) func() int {
	g := f
	return g
}

func Later(f func() int) {
	defer fmt.Println("later", f())
	fmt.Println("in Later")
}

func Spawn(f func() int) int {
	r := make(chan int)
	go func() { r <- f() }()
	return <-r
}

func Pass(f func() int) int { return Twice(func() int { return f() }) }

var calls int

func next() int {
	calls++
	return calls
}

// Each run of the code is a call of the function body: its named result starts at zero.
func counter(step int) (n int) {
	for range 3 {
		n = func() (n int) {
			n += step
			return
		}()
	}
	return
}

func sums(xs []int) (total int, count int) {
	for _, x := range xs {
		total += x
		count++
	}
	return
}

func held(a int) (r int) {
	var wg sync.WaitGroup
	wg.Go(func() { r = a * 2 })
	wg.Wait()
	return
}

func main() {
	x := 0
	fmt.Println(Twice(func() int { return func() int { x++; return x }() }), x)

	y := 1
	k := Keep(func() int { return y * 10 })
	y = 5
	fmt.Println(k(), k())

	sum := 0
	for i := 0; i < 5; i++ {
		sum += Twice(func() int { return i + sum })
	}
	fmt.Println(sum)

	z := 3
	Later(func() int { return z * 2 })
	z = 100
	fmt.Println("z", z)

	w := 2
	fmt.Println(Spawn(func() int { return w * 3 }), Pass(func() int { return next() }), calls)

	fmt.Println(counter(2))
	fmt.Println(sums([]int{1, 2, 3, 4}))
	fmt.Println(held(21))

	n := 10
	f := func() int { return Twice(func() int { return n + 1 }) }
	n = 20
	fmt.Println(f())

	a, b := 0, 0
	inc := func() { b++ }
	fmt.Println(Twice(func() int { return func() int { a++; inc(); return a + b }() }), a, b)
}
