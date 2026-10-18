// phase: 05

// Every way to reach recover: closure, named function, method value, method expression, interface method (value), func variable, returned closure, nested helpers, re-panic.
package main

import (
	"errors"
	"fmt"
)

type H struct{ name string }

func (h H) Recover(tag string) {
	fmt.Printf("%s %s: %v\n", tag, h.name, recover())
}
func (h *H) PRecover(tag string) {
	fmt.Printf("%s %s: %v\n", tag, h.name, recover())
}

type Rec interface{ Recover(string) }

func plain() { fmt.Println("plain:", recover()) }
func helper() any { return recover() }
func viaHelper() {
	r := helper()
	fmt.Println("via helper:", r)
}
func mkRec() func() {
	return func() { fmt.Println("mk:", recover()) }
}
func try(name string, f func()) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println(name, "escaped:", r)
		}
	}()
	f()
}


func main() {
	try("direct closure", func() {
		defer func() { fmt.Println("closure:", recover()) }()
		panic("p1")
	})
	try("named func", func() {
		defer plain()
		panic("p2")
	})
	try("helper from deferred closure", func() {
		defer func() { fmt.Println("closure got:", helper()) }()
		panic("p3")
	})
	try("deferred func calls helper directly", func() {
		defer viaHelper()
		panic("p4")
	})
	try("method value", func() {
		h := H{"hv"}
		f := h.Recover
		defer f("mv")
		panic("p5")
	})
	try("ptr method", func() {
		h := &H{"hp"}
		defer h.PRecover("pm")
		panic("p6")
	})
	try("method expr", func() {
		defer H.Recover(H{"me"}, "mexpr")
		panic("p7")
	})
	try("iface method", func() {
		var r Rec = H{"if"}
		defer r.Recover("im")
		panic("p8")
	})
	try("iface method value", func() {
		var r Rec = H{"ifv"}
		f := r.Recover
		defer f("imv")
		panic("p9")
	})
	try("func var", func() {
		f := plain
		defer f()
		panic("p10")
	})
	try("returned closure", func() {
		defer mkRec()()
		panic("p11")
	})
	try("defer recover() directly", func() {
		defer recover()
		panic("p12")
	})
	try("recover in nested closure of deferred", func() {
		defer func() {
			func() { fmt.Println("nested:", recover()) }()
		}()
		panic("p13")
	})
	try("re-panic", func() {
		defer func() {
			r := recover()
			panic(fmt.Sprint("re-", r))
		}()
		panic("p15")
	})
	try("panic in defer replaces", func() {
		defer func() { fmt.Println("outer defer:", recover()) }()
		defer func() { panic("second") }()
		panic("first")
	})
	try("recover twice", func() {
		defer func() {
			fmt.Println("first:", recover())
			fmt.Println("second:", recover())
		}()
		panic("p16")
	})
	try("recover when not panicking", func() {
		defer func() { fmt.Println("none:", recover()) }()
	})
	try("error value", func() {
		defer func() {
			r := recover()
			err, ok := r.(error)
			fmt.Println(ok, err, errors.Is(err, errSentinel))
		}()
		panic(fmt.Errorf("wrapped: %w", errSentinel))
	})
	try("runtime error", func() {
		defer func() {
			r := recover()
			_, isErr := r.(error)
			fmt.Printf("%T %v %v\n", r, isErr, r)
		}()
		var m map[int]int
		m[0] = 1
	})
	fmt.Println("done")
}

var errSentinel = errors.New("sentinel")
