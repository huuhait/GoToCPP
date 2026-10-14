// phase: 03

// Closures that outlive the functions that made them, and func values that a call changes or
// drops while it runs: a closure that sets its own variable to nil and goes on using what it
// captured (the call keeps the closure alive), the same through a struct field, many closures
// made in a loop, and a chain of 100,000 closures, each calling the one before it, which is
// freed without deep recursion. (Run with -asan: closures and boxes are freed exactly once.)
package main

import "fmt"

var self func() int

type holder struct {
	f func(string) string
}

func makeSelf() {
	data := []int{1, 2, 3}
	name := "self"
	self = func() int {
		self = nil // drops the last other owner of this closure
		total := 0
		for _, v := range data {
			total += v
		}
		return total + len(name)
	}
}

func adders(n int) []func(int) int {
	var fs []func(int) int
	for i := range n {
		base := i * 10
		fs = append(fs, func(x int) int { return x + base + i })
	}
	return fs
}

func chain(n int) func() int {
	f := func() int { return 0 }
	for i := 0; i < n; i++ {
		prev := f
		f = func() int { return prev() + 1 }
	}
	return f
}

func main() {
	makeSelf()
	fmt.Println(self(), self == nil)

	h := &holder{}
	suffix := "!"
	h.f = func(s string) string {
		h.f = nil
		return s + suffix
	}
	fmt.Println(h.f("done"), h.f == nil)

	fs := adders(5)
	sum := 0
	for _, f := range fs {
		sum += f(1)
	}
	fmt.Println(len(fs), sum)

	c := chain(1000)
	fmt.Println(c())
	long := chain(100000)
	long = nil
	fmt.Println(long == nil)
}

// Output:
// 10 true
// done! true
// 5 115
// 1000
// true
