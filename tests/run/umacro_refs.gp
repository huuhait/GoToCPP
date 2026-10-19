// phase: 08

// Lambdas for lazy arguments and wrapped code capture the caller's variables by reference when the macro
// only calls them where they stand (step 10.9); when the macro keeps, defers, starts or passes them on, they
// capture as every closure does. Either way a variable has Go's meaning: the closure sees the
// variable as it is when it runs, and changes it. The twin (testdata/macro_twin/umacro_refs.go) is plain
// Go: the macros as functions that take func values.

package main

import (
	"fmt"
	"sync"
)

// Call runs f twice and only calls it: its lambda can capture by reference.
macro Twice(f lazy int) int { return f() + f() }

// Keep stores f: its lambda must capture like any closure.
macro Keep(f lazy int) func() int {
	g := f
	return g
}

// Later defers f.
macro Later(f lazy int) {
	defer fmt.Println("later", f())
	fmt.Println("in Later")
}

// Spawn starts a goroutine that calls f and waits.
macro Spawn(f lazy int) int {
	r := make(chan int)
	go func() { r <- f() }()
	return <-r
}

// Pass hands f to another macro.
macro Pass(f lazy int) int { return #Twice(f()) }

// Run calls the code once; Repeat calls it n times; Hold keeps it in a goroutine.
macro Run() { #body() }

macro Repeat(n int) {
	for range n {
		#body()
	}
}

macro Hold() {
	var wg sync.WaitGroup
	wg.Go(func() { #body() })
	wg.Wait()
}

var calls int

func next() int {
	calls++
	return calls
}

#Repeat(3)
func counter(step int) (n int) {
	n += step
	return
}

#Run()
func sums(xs []int) (total int, count int) {
	for _, x := range xs {
		total += x
		count++
	}
	return
}

#Hold()
func held(a int) (r int) {
	r = a * 2
	return
}

func main() {
	// the lambda changes the caller's variable: Twice calls it twice
	x := 0
	fmt.Println(#Twice(func() int { x++; return x }()), x)

	// the lambda sees the variable as it is when it runs
	y := 1
	k := #Keep(y * 10)
	y = 5
	fmt.Println(k(), k())

	// in a loop: the loop variable and a variable that the loop changes
	sum := 0
	for i := 0; i < 5; i++ {
		sum += #Twice(i + sum)
	}
	fmt.Println(sum)

	z := 3
	#Later(z * 2)
	z = 100
	fmt.Println("z", z)

	w := 2
	fmt.Println(#Spawn(w*3), #Pass(next()), calls)

	// wrapped code: Repeat runs it three times; each run starts with zero results, like a call
	fmt.Println(counter(2))
	fmt.Println(sums([]int{1, 2, 3, 4}))
	fmt.Println(held(21))

	// a reference lambda inside a closure that is stored and run later
	n := 10
	f := func() int { return #Twice(n + 1) }
	n = 20
	fmt.Println(f())

	// a variable that only a reference lambda changes, and one that also a plain closure changes
	a, b := 0, 0
	inc := func() { b++ }
	fmt.Println(#Twice(func() int { a++; inc(); return a + b }()), a, b)
}

// Output:
// 3 2
// 50 50
// 116
// in Later
// later 6
// z 100
// 6 3 2
// 2
// 10 4
// 42
// 42
// 6 2 2
