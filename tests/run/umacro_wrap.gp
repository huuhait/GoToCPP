// phase: 10

// Macros on top of functions, wrap style (step 10.9): a macro that uses #body() gets the function's
// code and decides how it runs: once (VM), under recover (Recover), on another goroutine
// (InThread), once in the program's life (Once), several times (Retry) or not at all (Skip). A
// return in the code sets the results of the function and ends #body(); the defers of the code run
// when #body() returns. The twin (testdata/macro_twin/umacro_wrap.go) is the same program with the
// code of each function moved into a func literal by hand.

package main

import (
	"fmt"
	"sync"
)

macro VM() {
	fmt.Println("vm enter", #func())
	defer fmt.Println("vm exit", #func())
	#body()
}

macro Recover() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("recovered in", #func()+":", r)
		}
	}()
	#body()
}

macro InThread() {
	done := make(chan struct{})
	go func() {
		defer close(done)
		#body()
	}()
	<-done
}

macro Once(o *sync.Once) {
	o.Do(func() { #body() })
}

macro Retry(n int) {
	for i := 1; i <= n; i++ {
		fmt.Println("attempt", i, "of", n)
		#body()
	}
}

macro Skip(skip bool) {
	if skip {
		fmt.Println("skipped", #func())
		return
	}
	#body()
}

macro Trace() func() {
	fmt.Println("enter", #func())
	return func() { fmt.Println("exit ", #func()) }
}

#Trace()
#VM()
func secret(key string) (int, error) {
	defer fmt.Println("body defer of secret")
	if key == "" {
		return 0, fmt.Errorf("empty key")
	}
	return len(key) * 42, nil
}

#Recover()
func risky(a, b int) int {
	return a / b
}

#InThread()
func square(n int) int {
	return n * n
}

var setups int

#Once(&setupOnce)
func setup(name string) int {
	setups++
	fmt.Println("setup", name)
	return setups
}

var setupOnce sync.Once

var calls int

#Retry(3)
func attempt() (n int) {
	calls++
	n = calls * 10
	return
}

#Skip(skip)
func maybe(skip bool, v int) (int, string) {
	return v, "ran"
}

type Counter struct{ n int }

#VM()
func (c *Counter) Inc() (total int, err error) {
	defer func() { total *= 2 }()
	c.n++
	if c.n > 2 {
		err = fmt.Errorf("too many: %d", c.n)
		return
	}
	return c.n, nil
}

#Recover()
func (c Counter) Safe(i int) (_ int, ok bool) {
	arr := []int{1, 2, 3}
	return arr[i], true
}

#VM()
func sum(xs ...int) (t int) {
	for _, x := range xs {
		t += x
	}
	return t
}

#Recover()
#VM()
func Max[T int | string](a, b T) T {
	if a > b {
		return a
	}
	return b
}

#VM()
func noResult(n *int) {
	if *n > 0 {
		*n--
		return
	}
	*n = 100
}

func main() {
	fmt.Println(secret("key"))
	fmt.Println(secret(""))

	fmt.Println(risky(8, 2))
	fmt.Println(risky(1, 0))

	// InThread from four goroutines at the same time.
	results := make([]int, 4)
	var wg sync.WaitGroup
	for i := range 4 {
		wg.Go(func() { results[i] = square(i + 1) })
	}
	wg.Wait()
	fmt.Println("squares:", results)

	// Once: the code runs the first time only; later calls get the zero values.
	fmt.Println(setup("a"), setup("b"), setup("c"), "setups:", setups)

	// Retry runs the code three times; the results are those of the last run.
	fmt.Println(attempt(), "calls:", calls)

	for _, skip := range []bool{false, true} {
		v, s := maybe(skip, 7)
		fmt.Printf("%d %q\n", v, s)
	}

	c := &Counter{}
	fmt.Println(c.Inc())
	fmt.Println(c.Inc())
	fmt.Println(c.Inc())
	fmt.Println(Counter{}.Safe(1))
	fmt.Println(Counter{}.Safe(5))

	fmt.Println(sum(), sum(1, 2, 3))
	fmt.Println(Max(3, 9), Max("b", "a"))
	k := 1
	noResult(&k)
	noResult(&k)
	fmt.Println(k)
}

// Output:
// enter main.secret
// vm enter main.secret
// body defer of secret
// vm exit main.secret
// exit  main.secret
// 126 <nil>
// enter main.secret
// vm enter main.secret
// body defer of secret
// vm exit main.secret
// exit  main.secret
// 0 empty key
// 4
// recovered in main.risky: runtime error: integer divide by zero
// 0
// squares: [1 4 9 16]
// setup a
// 1 0 0 setups: 1
// attempt 1 of 3
// attempt 2 of 3
// attempt 3 of 3
// 30 calls: 3
// 7 "ran"
// skipped main.maybe
// 0 ""
// vm enter main.(*Counter).Inc
// vm exit main.(*Counter).Inc
// 2 <nil>
// vm enter main.(*Counter).Inc
// vm exit main.(*Counter).Inc
// 4 <nil>
// vm enter main.(*Counter).Inc
// vm exit main.(*Counter).Inc
// 0 too many: 3
// 2 true
// recovered in main.Counter.Safe: runtime error: index out of range [5] with length 3
// 0 false
// vm enter main.sum
// vm exit main.sum
// vm enter main.sum
// vm exit main.sum
// 0 6
// vm enter main.Max[...]
// vm exit main.Max[...]
// vm enter main.Max[...]
// vm exit main.Max[...]
// 9 b
// vm enter main.noResult
// vm exit main.noResult
// vm enter main.noResult
// vm exit main.noResult
// 100
