// (twin) the built-in macros as runtime calls: fn() and ln() ask the Go runtime where they are called.
// gc names an inlined literal in a literal differently (go build -gcflags=-l: .1), so two names are literals.

package main

import (
	"fmt"
	"runtime"
)

type T struct{}
type V int

func plain() string { return fn() }

func (t *T) Ptr() string { return fn() }
func (t T) Val() string  { return fn() }
func (v V) Num() string  { return fn() }

func (t *T) Closure() string {
	f := func() string { return fn() }
	return f()
}

var pkgVar = fn()
var pkgClosure = func() string { return fn() }()
var pkgClosure2 = func() string { return fn() }()

const where = "macro_file_line_func.gp"
const lineNo = 30

var arr [4]int

func init() {
	fmt.Println("init0", fn())
	func() { fmt.Println("init0 closure", fn()) }()
}

func init() { fmt.Println("init1", fn()) }

func nested() []string {
	var out []string
	out = append(out, fn())
	out = append(out, func() string { return fn() }())
	out = append(out, func() string {
		a := func() string { return "main.nested.func2.1" }()
		b := func() string { return "main.nested.func2.2" }()
		return a + " " + b + " " + fn()
	}())
	g := func() string { return fn() }
	out = append(out, g())
	defer func() { out = append(out, fn()) }()
	return out
}

func lines() (int, int, int) {
	a := ln()
	b :=
		ln()
	return a, b, ln()
}

func main() {
	fmt.Println(pkgVar, pkgClosure, pkgClosure2)
	fmt.Println(plain(), (&T{}).Ptr(), T{}.Val(), V(1).Num(), (&T{}).Closure())
	fmt.Println(nested())
	fmt.Println(where, lineNo, len(arr))
	fmt.Println(lines())
	fmt.Println("macro_file_line_func.gp", ln(), fn())
	m := (&T{}).Ptr
	fmt.Println(m())
	fmt.Println(func() string { return fn() }())
	x := struct{ F func() string }{func() string { return fn() }}
	fmt.Println(x.F())
	func() { defer func() { fmt.Println(fn()) }() }()
}

// fn is #func(): the name of the function that called it, as the runtime knows it (also for inlined code).
func fn() string {
	var pc [1]uintptr
	runtime.Callers(2, pc[:])
	f, _ := runtime.CallersFrames(pc[:]).Next()
	return f.Function
}

// ln is #line(): the line of the call.
func ln() int {
	var pc [1]uintptr
	runtime.Callers(2, pc[:])
	f, _ := runtime.CallersFrames(pc[:]).Next()
	return f.Line
}
