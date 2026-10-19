// phase: 08
package main

import "fmt"

func seq(n int) func(func(int) bool) {
	return func(yield func(int) bool) {
		for i := 0; i < n; i++ {
			if !yield(i) {
				return
			}
		}
	}
}

// the iterator itself calls recover: it is not the deferred function either
func recIter(yield func(int) bool) {
	fmt.Println("  iterator recover:", recover())
	yield(7)
}

func helper() any { return recover() }

func run(name string, f func()) {
	defer func() { fmt.Println(name, "escaped:", recover()) }()
	f()
}

// A deferred named function whose body has the loop.
func deferredNamed() {
	for i := range seq(2) {
		fmt.Println("  deferredNamed body", i, "recover:", recover())
	}
	fmt.Println("  deferredNamed after loop recover:", recover())
}

func t1() {
	defer deferredNamed()
	panic("t1")
}

// The body calls a helper that calls recover: nil as always (the helper is not deferred).
func t2() {
	defer func() {
		for i := range seq(1) {
			fmt.Println("  helper in body:", helper(), i)
		}
		fmt.Println("  helper after loop:", helper())
		fmt.Println("  recover after loop:", recover())
	}()
	panic("t2")
}

// The iterator function calls recover.
func t3() {
	defer func() {
		for i := range recIter {
			fmt.Println("  body", i, "recover:", recover())
		}
		fmt.Println("  after:", recover())
	}()
	panic("t3")
}

// recover in the range clause is evaluated in the function itself: it recovers.
func t4() {
	defer func() {
		wrap := func(v any) func(func(int) bool) {
			fmt.Println("  range expression got:", v)
			return seq(2)
		}
		for i := range wrap(recover()) {
			fmt.Println("  body", i, "recover:", recover())
		}
	}()
	panic("t4")
}

// A defer inside the body belongs to the function; when the function panics, that deferred
// function is the one the panic runs, so its recover() works.
func t5() {
	for i := range seq(3) {
		defer func() {
			fmt.Println("  body-defer", i, "recover:", recover())
		}()
	}
	panic("t5")
}

// A recover() in an argument of a defer in the body is evaluated in the body: nil.
func t6() {
	defer func() { fmt.Println("  t6 outer:", recover()) }()
	for i := range seq(1) {
		defer fmt.Println("  arg recover:", recover(), i)
	}
	panic("t6")
}

// Two loops deep, and a func literal called in the body, and one deferred in the body.
func t7() {
	defer func() {
		for i := range seq(2) {
			for j := range seq(2) {
				func() {
					fmt.Println("  literal in body:", recover(), i, j)
				}()
				if v := recover(); v != nil {
					fmt.Println("  recovered in nested body", v)
				}
			}
		}
		fmt.Println("  t7 after loops:", recover())
	}()
	panic("t7")
}

// Not panicking at all.
func t8() {
	for i := range seq(2) {
		fmt.Println("  quiet body", i, "recover:", recover())
	}
}

// The panic comes from the body; the function's own deferred function recovers.
func t9() (r string) {
	defer func() {
		r = fmt.Sprint("recovered ", recover())
	}()
	for i := range seq(5) {
		if i == 2 {
			panic("from body")
		}
		_ = recover()
	}
	return "unreachable"
}

// A body that calls recover while the deferred function's own deferred call also recovers.
func t10() {
	defer func() { fmt.Println("  t10 outer:", recover()) }()
	defer func() {
		defer func() { fmt.Println("  t10 inner deferred:", recover()) }()
		for i := range seq(2) {
			fmt.Println("  t10 body", i, recover())
		}
	}()
	panic("t10")
}

// Another goroutine runs the body.
func t11() {
	defer func() {
		done := make(chan int)
		it := func(yield func(int) bool) {
			go func() {
				yield(1)
				done <- 1
			}()
			<-done
		}
		for i := range it {
			fmt.Println("  goroutine body", i, "recover:", recover())
		}
		fmt.Println("  t11 after:", recover())
	}()
	panic("t11")
}

func main() {
	run("t1", t1)
	run("t2", t2)
	run("t3", t3)
	run("t4", t4)
	run("t5", t5)
	run("t6", t6)
	run("t7", t7)
	run("t8", t8)
	fmt.Println(t9())
	run("t10", t10)
	run("t11", t11)
}

// Output:
//   deferredNamed body 0 recover: <nil>
//   deferredNamed body 1 recover: <nil>
//   deferredNamed after loop recover: t1
// t1 escaped: <nil>
//   helper in body: <nil> 0
//   helper after loop: <nil>
//   recover after loop: t2
// t2 escaped: <nil>
//   iterator recover: <nil>
//   body 7 recover: <nil>
//   after: t3
// t3 escaped: <nil>
//   range expression got: t4
//   body 0 recover: <nil>
//   body 1 recover: <nil>
// t4 escaped: <nil>
//   body-defer 2 recover: t5
//   body-defer 1 recover: <nil>
//   body-defer 0 recover: <nil>
// t5 escaped: <nil>
//   arg recover: <nil> 0
//   t6 outer: t6
// t6 escaped: <nil>
//   literal in body: <nil> 0 0
//   literal in body: <nil> 0 1
//   literal in body: <nil> 1 0
//   literal in body: <nil> 1 1
//   t7 after loops: t7
// t7 escaped: <nil>
//   quiet body 0 recover: <nil>
//   quiet body 1 recover: <nil>
// t8 escaped: <nil>
// recovered from body
//   t10 body 0 <nil>
//   t10 body 1 <nil>
//   t10 inner deferred: <nil>
//   t10 outer: t10
// t10 escaped: <nil>
//   goroutine body 1 recover: <nil>
//   t11 after: t11
// t11 escaped: <nil>
