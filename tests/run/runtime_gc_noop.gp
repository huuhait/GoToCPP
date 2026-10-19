// phase: 08

// GC, KeepAlive, SetFinalizer and LockOSThread are callable, and the program goes on as before
// (gocpp frees memory by reference counting: GC does nothing and a finalizer never runs, which
// Go allows: it promises no finalizer ever runs).
package main

import (
	"fmt"
	"runtime"
)

type T struct{ n int }

func main() {
	t := &T{7}
	runtime.SetFinalizer(t, func(t *T) { fmt.Println("finalized") })
	runtime.GC()
	runtime.GC()
	runtime.KeepAlive(t)
	runtime.SetFinalizer(t, nil)
	runtime.LockOSThread()
	runtime.LockOSThread()
	runtime.UnlockOSThread()
	runtime.UnlockOSThread()
	fmt.Println(t.n)
}

// Output:
// 7
