// phase: 07

// The body of a range-over-func loop has a defer: it belongs to the function with the loop. A
// yield called after that function returned must panic before it registers a deferred call on the
// dead frame (ASan-clean).
package main

import "fmt"

var saved []func(string) bool

func iter(yield func(string) bool) {
	saved = append(saved, yield)
	yield("first")
	yield("second")
}

func work(tag string) (n int) {
	defer func() { fmt.Println(tag, "function deferred, n =", n) }()
	for s := range iter {
		defer fmt.Println(tag, "body deferred", s)
		n += len(s)
	}
	return n * 10
}

func try(i int, f func(string) bool) {
	defer func() { fmt.Println("late", i, "->", recover()) }()
	f("late")
	fmt.Println("not reached")
}

func main() {
	fmt.Println(work("a"))
	fmt.Println(work("b"))
	for i, f := range saved {
		try(i, f)
	}
	fmt.Println("done")
}

// Output:
// a body deferred second
// a body deferred first
// a function deferred, n = 110
// 110
// b body deferred second
// b body deferred first
// b function deferred, n = 110
// 110
// late 0 -> runtime error: range function continued iteration after whole loop exit
// late 1 -> runtime error: range function continued iteration after whole loop exit
// done
