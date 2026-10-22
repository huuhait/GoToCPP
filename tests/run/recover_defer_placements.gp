// phase: 05

// recover in every placement of a deferred call: which ones stop the panic is Go's rule (recover
// is called directly by the deferred function that the panic runs), checked against real Go.
// `defer recover()` inside a function that is itself a deferred call of the panic does stop it
// (cases 5, 20, 26), and `defer recover()` in the panicking function does not (case 1).
package main

import "fmt"

func report(name string, f func()) {
	defer func() {
		fmt.Println(name, "outer", recover())
	}()
	f()
}

func F() { recover() }
func Fskip(skip bool) {
	defer recover()
	if !skip {
		recover()
	}
}
func G() { F() }
func H() { defer F() }
func Pr(x any) { fmt.Println("pr", x) }
func Rp() any { return recover() }
func RpPass() { fmt.Println("rp", recover()) }
func DeferRecoverInside() {
	defer recover()
}
func NestedCall() {
	func() {
		recover()
	}()
}

type T struct{}

func (T) M()   { recover() }
func (*T) PM() { recover() }

type I interface{ M() }

func main() {
	report("1 defer recover()", func() {
		defer recover()
		panic("a")
	})
	report("2 defer func(){recover()}()", func() {
		defer func() { recover() }()
		panic("b")
	})
	report("3 defer F()", func() {
		defer F()
		panic("c")
	})
	report("4 defer G()", func() {
		defer G()
		panic("d")
	})
	report("5 defer Fskip(true)", func() {
		defer Fskip(true)
		panic("e")
	})
	report("6 defer Fskip(false)", func() {
		defer Fskip(false)
		panic("f")
	})
	report("7 method value", func() {
		var t T
		defer t.M()
		panic("g")
	})
	report("8 ptr method value", func() {
		t := &T{}
		defer t.PM()
		panic("h")
	})
	report("9 method expr", func() {
		defer T.M(T{})
		panic("i")
	})
	report("10 iface method", func() {
		var i I = T{}
		defer i.M()
		panic("j")
	})
	report("11 bound method value stored", func() {
		f := T{}.M
		defer f()
		panic("k")
	})
	report("12 nested closure", func() {
		defer func() {
			func() { recover() }()
		}()
		panic("l")
	})
	report("13 defer NestedCall", func() {
		defer NestedCall()
		panic("m")
	})
	report("14 recover after re-panic", func() {
		defer func() {
			recover()
		}()
		defer func() {
			defer func() { fmt.Println("14 inner", recover()) }()
			panic("n2")
		}()
		panic("n1")
	})
	report("15 re-panic in deferred, outer recovers", func() {
		defer func() { fmt.Println("15 rec", recover()) }()
		defer func() { panic("o2") }()
		panic("o1")
	})
	report("16 recover value passed to defer arg", func() {
		defer Pr(recover())
		panic("p")
	})
	report("17 defer Rp()", func() {
		defer Rp()
		panic("q")
	})
	report("18 defer RpPass()", func() {
		defer RpPass()
		panic("r")
	})
	report("19 defer func literal calls Rp", func() {
		defer func() { fmt.Println("19", Rp()) }()
		panic("s")
	})
	report("20 DeferRecoverInside as deferred", func() {
		defer DeferRecoverInside()
		panic("t")
	})
	report("21 recover without panic", func() {
		fmt.Println("21", recover())
		defer func() { fmt.Println("21b", recover()) }()
	})
	report("22 func value var", func() {
		g := F
		defer g()
		panic("u")
	})
	report("23 defer func literal with args", func() {
		defer func(x int) { recover() }(1)
		panic("v")
	})
	report("24 defer in loop", func() {
		for i := 0; i < 2; i++ {
			defer func() { fmt.Println("24", i, recover()) }()
		}
		panic("w")
	})
	report("25 two panics, recover each", func() {
		defer func() { fmt.Println("25a", recover()) }()
		defer func() {
			defer func() { fmt.Println("25b", recover()) }()
			panic("x2")
		}()
		panic("x1")
	})
	report("26 defer that calls defer recover", func() {
		defer func() {
			defer recover()
		}()
		panic("y")
	})
	report("27 defer F in deferred func", func() {
		defer func() {
			defer F()
		}()
		panic("z")
	})
	report("28 recover in function called by recovered-after", func() {
		defer func() {
			recover()
			defer F()
			panic("z2")
		}()
		panic("z1")
	})
}

// Output:
// 1 defer recover() outer a
// 2 defer func(){recover()}() outer <nil>
// 3 defer F() outer <nil>
// 4 defer G() outer d
// 5 defer Fskip(true) outer <nil>
// 6 defer Fskip(false) outer <nil>
// 7 method value outer <nil>
// 8 ptr method value outer <nil>
// 9 method expr outer <nil>
// 10 iface method outer <nil>
// 11 bound method value stored outer <nil>
// 12 nested closure outer l
// 13 defer NestedCall outer m
// 14 inner n2
// 14 recover after re-panic outer <nil>
// 15 rec o2
// 15 re-panic in deferred, outer recovers outer <nil>
// pr <nil>
// 16 recover value passed to defer arg outer p
// 17 defer Rp() outer <nil>
// rp r
// 18 defer RpPass() outer <nil>
// 19 <nil>
// 19 defer func literal calls Rp outer s
// 20 DeferRecoverInside as deferred outer <nil>
// 21 <nil>
// 21b <nil>
// 21 recover without panic outer <nil>
// 22 func value var outer <nil>
// 23 defer func literal with args outer <nil>
// 24 1 w
// 24 0 <nil>
// 24 defer in loop outer <nil>
// 25b x2
// 25a x1
// 25 two panics, recover each outer <nil>
// 26 defer that calls defer recover outer <nil>
// 27 defer F in deferred func outer z
// 28 recover in function called by recovered-after outer <nil>
