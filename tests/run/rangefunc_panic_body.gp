// phase: 07

package main

import "fmt"

func Cnt(n int) func(yield func(int) bool) {
	return func(yield func(int) bool) {
		defer fmt.Println("iterator deferred")
		for i := range n {
			if !yield(i) {
				return
			}
		}
	}
}

func try(name string, f func()) {
	defer func() {
		r := recover()
		fmt.Println(name, "recovered:", r)
	}()
	f()
}

// The iterator recovers the body's panic and panics again.
func Rethrow(yield func(int) bool) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("iterator saw", r)
			panic(fmt.Sprint("again: ", r))
		}
	}()
	yield(1)
}

func main() {
	try("body panic", func() {
		for x := range Cnt(3) {
			fmt.Println("body", x)
			if x == 1 {
				panic("boom")
			}
		}
	})
	try("nested", func() {
		for x := range Cnt(2) {
			for y := range Cnt(2) {
				if x == 1 && y == 1 {
					panic(fmt.Sprint("deep ", x, y))
				}
			}
		}
	})
	try("rethrow", func() {
		for x := range Rethrow {
			fmt.Println("body", x)
			panic("first")
		}
	})
	try("iterator panics", func() {
		for x := range func(yield func(int) bool) {
			yield(1)
			panic("iterator boom")
		} {
			fmt.Println("body", x)
		}
	})
	try("error value", func() {
		for x := range Cnt(2) {
			_ = x
			var m map[string]int
			m["a"] = 1
		}
	})
	fmt.Println("done")
}

// Output:
// body 0
// body 1
// iterator deferred
// body panic recovered: boom
// iterator deferred
// iterator deferred
// iterator deferred
// nested recovered: deep 1 1
// body 1
// iterator saw first
// rethrow recovered: again: first
// body 1
// iterator panics recovered: iterator boom
// iterator deferred
// error value recovered: assignment to entry in nil map
// done
