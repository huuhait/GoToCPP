// phase: 05

// A lazy argument that a macro keeps outlives the function that wrote it (step 10.13): its lambda must
// capture the variables like any closure (a copy, or the box of a variable that the function changes), not by
// reference to the stack, which a macro that only calls the argument may do (umacro_refs.gp). The twin
// (testdata/macro_twin/umacro_escape.go) is plain Go with func values.

package main

import "fmt"

// Keep stores f and gives it back.
macro Keep(f lazy int) func() int {
	g := f
	return g
}

// A Holder keeps lazy values in a struct and a slice.
type Holder struct {
	fs []func() int
}

func (h *Holder) add(f func() int) { h.fs = append(h.fs, f) }

func (h *Holder) sum() (total int) {
	for _, f := range h.fs {
		total += f()
	}
	return total
}

// once: the variable dies when the function returns; the stored lazy still has it.
func once() func() int {
	v := 7
	k := #Keep(v * 10)
	v = 8
	return k
}

// shared: the stored lazy and a closure share the variable n.
func shared() (get func() int, inc func()) {
	n := 0
	get = #Keep(n + 1)
	inc = func() { n++ }
	return get, inc
}

// loop: each iteration has its own variable.
func loop() *Holder {
	h := &Holder{}
	for i := 0; i < 3; i++ {
		h.add(#Keep(i * i))
	}
	return h
}

func clobber() int {
	var a [64]int
	for i := range a {
		a[i] = -1
	}
	return a[3]
}

func main() {
	k := once()
	clobber()
	fmt.Println(k(), k())

	get, inc := shared()
	clobber()
	fmt.Println(get())
	inc()
	inc()
	fmt.Println(get())

	h := loop()
	clobber()
	fmt.Println(h.sum())
}

// Output:
// 80 80
// 1
// 3
// 5
