// phase: 07

package main

import (
	"errors"
	"fmt"
)

type runtimeError interface {
	error
	RuntimeError()
}

func try(name string, f func()) {
	defer func() {
		r := recover()
		err, ok := r.(error)
		_, isRT := r.(runtimeError)
		fmt.Printf("%s: %v (error %v, runtime.Error %v, %T)\n", name, r, ok, isRT, r)
		_ = errors.Is
		_ = err
	}()
	f()
}

var saved func(int) bool

func main() {
	try("continued after break", func() {
		for x := range func(yield func(int) bool) { yield(1); yield(2) } {
			_ = x
			break
		}
	})
	try("continued after return", func() {
		for x := range func(yield func(int) bool) { yield(1); yield(2) } {
			_ = x
			return
		}
	})
	try("late yield", func() {
		for x := range func(yield func(int) bool) { saved = yield; yield(1) } {
			_ = x
		}
		saved(3)
	})
	try("continued after panic", func() {
		for x := range func(yield func(int) bool) {
			func() {
				defer func() { recover() }()
				yield(1)
			}()
			yield(2)
		} {
			_ = x
			panic("p")
		}
	})
	try("missing panic", func() {
		for x := range func(yield func(int) bool) {
			defer func() { recover() }()
			yield(1)
		} {
			_ = x
			panic("p")
		}
	})
	try("nested late yield", func() {
		var inner func() bool
		for x := range func(yield func(int) bool) { yield(1); yield(2) } {
			for y := range func(yield func(int) bool) { yield(10); if inner != nil { inner() } } {
				_ = y
				if x == 1 {
					inner = func() bool { return saved(y) }
				}
			}
		}
	})
	fmt.Println("done")
}

// Output:
// continued after break: runtime error: range function continued iteration after function for loop body returned false (error true, runtime.Error true, runtime.errorString)
// continued after return: runtime error: range function continued iteration after function for loop body returned false (error true, runtime.Error true, runtime.errorString)
// late yield: runtime error: range function continued iteration after whole loop exit (error true, runtime.Error true, runtime.errorString)
// continued after panic: runtime error: range function continued iteration after loop body panic (error true, runtime.Error true, runtime.errorString)
// missing panic: runtime error: range function recovered a loop body panic and did not resume panicking (error true, runtime.Error true, runtime.errorString)
// nested late yield: runtime error: range function continued iteration after loop body panic (error true, runtime.Error true, runtime.errorString)
// done
