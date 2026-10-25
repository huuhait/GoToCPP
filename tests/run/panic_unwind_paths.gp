// phase: 08

// Panics that are thrown and recovered on every way the runtime knows: in a callee of the function
// that recovers, through functions whose deferred calls run and let the panic go on, from a deep
// recursion, as runtime errors whose text is made where they happen (index and slice bounds with
// the largest and the negative numbers), a deferred call that panics again, a runtime error in a
// deferred call while a panic runs, and Goexit. Each way runs many times, so that a -release build
// walks the stacks with the rules it kept the first time (src/os/linux/trace.cpp).
package main

import (
	"errors"
	"fmt"
	"runtime"
)

var errBad = errors.New("bad")

var ran int

func thrower(i int) {
	if i%2 == 0 {
		panic(i)
	}
	panic(errBad)
}

func callee(i int) (r string) {
	defer func() {
		if x := recover(); x != nil {
			r = fmt.Sprintf("%T", x)
		}
	}()
	thrower(i)
	return "none"
}

func mid(i int) {
	defer func() { ran++ }()
	thrower(i)
}

func mid2(i int) {
	s := fmt.Sprint(i) // a local to clean up on the way
	defer func() { ran += len(s) - len(s) }()
	mid(i)
}

func through(i int) (r int) {
	defer func() {
		if x := recover(); x != nil {
			r = 1
		}
	}()
	mid2(i)
	return 0
}

func deep(i, n int) int {
	if n == 0 {
		thrower(i)
	}
	if n%7 == 0 {
		defer func() { ran++ }()
	}
	return deep(i, n-1) + 1
}

func deepr(i int) (r int) {
	defer func() {
		if recover() != nil {
			r = 1
		}
	}()
	return deep(i, 40)
}

func text(f func()) (s string) {
	defer func() {
		x := recover()
		err, ok := x.(runtime.Error)
		s = fmt.Sprintf("%T %v: %v", x, ok, err)
	}()
	f()
	return "no panic"
}

//go:noinline
func at(s []int, i int) int { return s[i] }

//go:noinline
func atu(s []int, i uint) int { return s[i] }

func repanic(i int) (r string) {
	defer func() {
		r = fmt.Sprint(recover())
	}()
	func() {
		defer func() {
			x := recover()
			panic(fmt.Sprint("again ", x))
		}()
		thrower(i)
	}()
	return "none"
}

func chain() (r string) {
	defer func() {
		r = fmt.Sprint(recover())
	}()
	defer func() {
		var m map[string]int
		m["x"] = 1 // a runtime error while the panic runs
	}()
	panic("first")
}

func main() {
	counts := map[string]int{}
	for i := 0; i < 3000; i++ {
		counts[callee(i)]++
	}
	fmt.Println(counts)
	n := 0
	for i := 0; i < 3000; i++ {
		n += through(i)
	}
	fmt.Println(n, ran)
	ran = 0
	n = 0
	for i := 0; i < 500; i++ {
		n += deepr(i)
	}
	fmt.Println(n, ran)

	s := []int{1, 2, 3}
	k, big, neg, five, ten := 2, 1<<62, -1, 5, 10
	one := 1
	for range 2 {
		fmt.Println(text(func() { at(s, five) }))
		fmt.Println(text(func() { at(s, neg) }))
		fmt.Println(text(func() { at(s, big) }))
		fmt.Println(text(func() { atu(s, 1<<63) }))
		fmt.Println(text(func() { _ = s[k:one] }))
		fmt.Println(text(func() { _ = s[:ten] }))
		fmt.Println(text(func() { _ = s[five:] }))
		fmt.Println(text(func() { _ = s[one:k:ten] }))
		fmt.Println(text(func() { _ = s[neg:] }))
		fmt.Println(text(func() { _ = [4]int(s) }))
		fmt.Println(text(func() { _ = 10 / (k - 2) }))
	}
	for i := 0; i < 2; i++ {
		fmt.Println(repanic(i))
	}
	fmt.Println(chain())

	done := make(chan string)
	go func() {
		defer func() { done <- fmt.Sprint("goexit: recover() = ", recover()) }()
		runtime.Goexit()
	}()
	fmt.Println(<-done)
}

// Output:
// map[*errors.errorString:1500 int:1500]
// 3000 3000
// 500 2500
// runtime.boundsError true: runtime error: index out of range [5] with length 3
// runtime.boundsError true: runtime error: index out of range [-1]
// runtime.boundsError true: runtime error: index out of range [4611686018427387904] with length 3
// runtime.boundsError true: runtime error: index out of range [9223372036854775808] with length 3
// runtime.boundsError true: runtime error: slice bounds out of range [2:1]
// runtime.boundsError true: runtime error: slice bounds out of range [:10] with capacity 3
// runtime.boundsError true: runtime error: slice bounds out of range [5:3]
// runtime.boundsError true: runtime error: slice bounds out of range [::10] with capacity 3
// runtime.boundsError true: runtime error: slice bounds out of range [-1:]
// runtime.boundsError true: runtime error: cannot convert slice with length 3 to array or pointer to array with length 4
// runtime.errorString true: runtime error: integer divide by zero
// runtime.boundsError true: runtime error: index out of range [5] with length 3
// runtime.boundsError true: runtime error: index out of range [-1]
// runtime.boundsError true: runtime error: index out of range [4611686018427387904] with length 3
// runtime.boundsError true: runtime error: index out of range [9223372036854775808] with length 3
// runtime.boundsError true: runtime error: slice bounds out of range [2:1]
// runtime.boundsError true: runtime error: slice bounds out of range [:10] with capacity 3
// runtime.boundsError true: runtime error: slice bounds out of range [5:3]
// runtime.boundsError true: runtime error: slice bounds out of range [::10] with capacity 3
// runtime.boundsError true: runtime error: slice bounds out of range [-1:]
// runtime.boundsError true: runtime error: cannot convert slice with length 3 to array or pointer to array with length 4
// runtime.errorString true: runtime error: integer divide by zero
// again 0
// again bad
// assignment to entry in nil map
// goexit: recover() = <nil>
