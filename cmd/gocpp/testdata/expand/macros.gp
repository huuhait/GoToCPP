// Macros shows what macro declarations and uses become.
package main

import (
	"fmt"
	"sync"
)

// Assert panics when cond is false; msg is only built then.
macro Assert(cond bool, msg lazy string) {
	if !cond {
		panic(fmt.Sprintf("%s:%d: assertion failed: %s: %s", #file(), #line(), #text(cond), msg()))
	}
}

// Dbg returns v.
macro Dbg[T any](v T) T {
	fmt.Printf("[%s:%d] %s = %v\n", #file(), #line(), #text(v), v)
	return v
}

macro Trace() func() {
	fmt.Println("enter", #func())
	return func() { fmt.Println("exit ", #func()) }
}

macro Locked(mu *sync.Mutex) func() {
	mu.Lock()
	return mu.Unlock
}

macro Recover() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("recovered in", #func()+":", r)
		}
	}()
	#body()
}

var mu sync.Mutex

func   spaced( a  int ) int   {  return a }

// double has two macros on top: Trace is applied first (the outer one).
#Trace()
#Locked(&mu)
func double(n int) int {
	// the code
	#Assert(n < 100, fmt.Sprint("too big: ", n))
	return n * 2
}

#Recover()
func ratio(a, b int) (q int, err error) {
	q = a / b
	return
}

func main() {
	// a use inside code, with a nested one
	x := #Dbg(double(#Dbg(2)))
	fmt.Println(x, spaced(1))
	fmt.Println(ratio(1, 0))
}
