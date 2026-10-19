// phase: 10

// Macros on top of functions, start style (step 10.9): the call becomes the first statement of the
// function; a func() it returns is deferred (unless it is nil), so it runs when the function ends,
// also on a panic, after the function's own defers. Several macros apply from top to bottom. The
// arguments are evaluated inside the function, so they can use the parameters and the receiver. The
// twin (testdata/macro_twin/umacro_start.go) is the same program with the macros written out.

package main

import (
	"fmt"
	"sync"
)

// Trace prints when the function starts and when it ends.
macro Trace() func() {
	fmt.Println("enter", #func())
	return func() { fmt.Println("exit ", #func()) }
}

// Locked holds mu while the function runs.
macro Locked(mu *sync.Mutex) func() {
	mu.Lock()
	return mu.Unlock
}

// Maybe does its work only when on is true; with nil there is nothing to run at the end.
macro Maybe(on bool) func() {
	if !on {
		return nil
	}
	fmt.Println("maybe on at line", #line())
	return func() { fmt.Println("maybe off in", #func()) }
}

// Note has no result: it just runs at the start.
macro Note(msg string) {
	fmt.Println("note:", msg, #file(), #line())
}

// Safe stops a panic of the function.
macro Safe() func() {
	return func() {
		if r := recover(); r != nil {
			fmt.Println("safe recovered:", r, "in", #func())
		}
	}
}

type Store struct {
	mu sync.Mutex
	m  map[string]int
}

#Locked(&s.mu)
func (s *Store) Put(k string, v int) {
	s.m[k] += v
}

#Trace()
func plain() string {
	fmt.Println("in plain")
	return "plain"
}

#Trace()
func panics() {
	defer fmt.Println("own defer of panics")
	panic("boom")
}

#Maybe(n > 0)
func maybe(n int) int {
	fmt.Println("in maybe", n)
	return n * 2
}

#Trace()
#Note("stacked")
#Maybe(true)
func stacked() (res int) {
	defer func() { res++ }()
	fmt.Println("in stacked")
	return 10
}

#Safe()
func safe(n int) (res int, err error) {
	res = n
	if n > 1 {
		panic(fmt.Sprint("n too big: ", n))
	}
	return res, nil
}

#Note("generic")
func Id[T any](v T) T { return v }

func main() {
	fmt.Println(plain())

	func() {
		defer func() { fmt.Println("main recovered:", recover()) }()
		panics()
	}()

	fmt.Println(maybe(2))
	fmt.Println(maybe(-1))

	fmt.Println(stacked())

	fmt.Println(safe(1))
	fmt.Println(safe(5))
	fmt.Println(Id("id"), Id(3))

	// Locked: 100 goroutines at the same time, one Put at a time.
	s := &Store{m: map[string]int{}}
	var wg sync.WaitGroup
	for i := range 100 {
		wg.Go(func() { s.Put(fmt.Sprint("k", i%3), i) })
	}
	wg.Wait()
	fmt.Println(s.m["k0"], s.m["k1"], s.m["k2"])
}

// Output:
// enter main.plain
// in plain
// exit  main.plain
// plain
// enter main.panics
// own defer of panics
// exit  main.panics
// main recovered: boom
// maybe on at line 73
// in maybe 2
// maybe off in main.maybe
// 4
// in maybe -1
// -2
// enter main.stacked
// note: stacked umacro_start.gp 80
// maybe on at line 81
// in stacked
// maybe off in main.stacked
// exit  main.stacked
// 11
// 1 <nil>
// safe recovered: n too big: 5 in main.safe
// 5 <nil>
// note: generic umacro_start.gp 97
// note: generic umacro_start.gp 97
// id 3
// 1683 1617 1650
