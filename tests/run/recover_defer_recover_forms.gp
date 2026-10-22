// phase: 05

// `defer recover()` in functions that a panic runs as deferred calls (methods, generic functions,
// conditional and repeated defers, named results, re-panics): it stops the panic exactly when the
// function that contains it is itself the deferred call of the panic. Checked against real Go.
package main

import "fmt"

func report(name string, f func()) {
	defer func() {
		fmt.Println(name, "outer", recover())
	}()
	f()
}

type T struct{}

func (T) M()   { defer recover() }
func (*T) PM() { defer recover() }

type I interface{ M() }

func Gen[X any](x X) { defer recover() }

func Cond(b bool) {
	if b {
		defer recover()
	}
}

func Loop() {
	for i := 0; i < 2; i++ {
		defer recover()
	}
}

func Twice() {
	defer recover()
	defer recover()
}

func RecoverFirst() {
	recover()
	defer recover()
}

func Seq(yield func(int) bool) { yield(1) }

func RangeBody() {
	for range Seq {
		defer recover()
	}
}

func Fn() (r int) {
	defer recover()
	r = 5
	return
}

func Both() {
	defer recover()
	defer func() { fmt.Println("both inner", recover()) }()
}

func Wrap() {
	defer func() {
		defer recover()
	}()
}

func main() {
	report("1 method M", func() { var t T; defer t.M(); panic("a") })
	report("2 ptr method", func() { t := &T{}; defer t.PM(); panic("b") })
	report("3 iface method", func() { var i I = T{}; defer i.M(); panic("c") })
	report("4 generic", func() { defer Gen(1); panic("d") })
	report("5 cond true", func() { defer Cond(true); panic("e") })
	report("6 cond false", func() { defer Cond(false); panic("f") })
	report("7 loop", func() { defer Loop(); panic("g") })
	report("8 twice", func() { defer Twice(); panic("h") })
	report("9 recover first", func() { defer RecoverFirst(); panic("i") })
	report("10 range body", func() { defer RangeBody(); panic("j") })
	report("11 named result", func() { defer Fn(); panic("k") })
	report("12 both", func() { defer Both(); panic("l") })
	report("13 wrap", func() { defer Wrap(); panic("m") })
	report("14 plain call no panic", func() { Twice(); fmt.Println("14 ok") })
	report("15 plain call in panic deferred closure", func() {
		defer func() { Twice() }()
		panic("n")
	})
	report("16 func value", func() { f := T.M; defer f(T{}); panic("o") })
	report("17 literal call", func() { defer func() { defer recover() }(); panic("p") })
	report("18 method value", func() { f := T{}.M; defer f(); panic("q") })
	report("19 re-panic after recover", func() {
		defer func() { fmt.Println("19 rec", recover()) }()
		defer func() {
			defer recover()
			panic("r2")
		}()
		panic("r1")
	})
	report("20 defer recover in main-level goroutine-less", func() {
		defer func() {
			defer func() { fmt.Println("20 inner", recover()) }()
			defer recover()
		}()
		panic("s")
	})
}

// Output:
// 1 method M outer <nil>
// 2 ptr method outer <nil>
// 3 iface method outer <nil>
// 4 generic outer <nil>
// 5 cond true outer <nil>
// 6 cond false outer f
// 7 loop outer <nil>
// 8 twice outer <nil>
// 9 recover first outer <nil>
// 10 range body outer j
// 11 named result outer <nil>
// both inner <nil>
// 12 both outer <nil>
// 13 wrap outer m
// 14 ok
// 14 plain call no panic outer <nil>
// 15 plain call in panic deferred closure outer n
// 16 func value outer <nil>
// 17 literal call outer <nil>
// 18 method value outer <nil>
// 19 rec r2
// 19 re-panic after recover outer <nil>
// 20 inner <nil>
// 20 defer recover in main-level goroutine-less outer <nil>
