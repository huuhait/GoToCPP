// phase: 05

// #file(), #line() and #func() (Phase 10 step 10.8): the place of the use, in functions,
// methods, closures, package-level initializers, init functions and const contexts. The
// names are those that Go's runtime prints for code that is not inlined (go build -gcflags=-l).

package main

import "fmt"

type T struct{}
type V int

func plain() string { return #func() }

func (t *T) Ptr() string { return #func() }
func (t T) Val() string  { return #func() }
func (v V) Num() string  { return #func() }

func (t *T) Closure() string {
	f := func() string { return #func() }
	return f()
}

var pkgVar = #func()
var pkgClosure = func() string { return #func() }()
var pkgClosure2 = func() string { return #func() }()

const where = #file()
const lineNo = #line()

var arr [#line() - 28]int

func init() {
	fmt.Println("init0", #func())
	func() { fmt.Println("init0 closure", #func()) }()
}

func init() { fmt.Println("init1", #func()) }

func nested() []string {
	var out []string
	out = append(out, #func())
	out = append(out, func() string { return #func() }())
	out = append(out, func() string {
		a := func() string { return #func() }()
		b := func() string { return #func() }()
		return a + " " + b + " " + #func()
	}())
	g := func() string { return #func() }
	out = append(out, g())
	defer func() { out = append(out, #func()) }()
	return out
}

func lines() (int, int, int) {
	a := #line()
	b :=
		#line()
	return a, b, #line()
}

func main() {
	fmt.Println(pkgVar, pkgClosure, pkgClosure2)
	fmt.Println(plain(), (&T{}).Ptr(), T{}.Val(), V(1).Num(), (&T{}).Closure())
	fmt.Println(nested())
	fmt.Println(where, lineNo, len(arr))
	fmt.Println(lines())
	fmt.Println(#file(), #line(), #func())
	m := (&T{}).Ptr
	fmt.Println(m())
	fmt.Println(func() string { return #func() }())
	x := struct{ F func() string }{func() string { return #func() }}
	fmt.Println(x.F())
	func() { defer func() { fmt.Println(#func()) }() }()
}

// Output:
// init0 main.init.0
// init0 closure main.init.0.func1
// init1 main.init.1
// main.init main.init.func1 main.init.func2
// main.plain main.(*T).Ptr main.T.Val main.V.Num main.(*T).Closure.func1
// [main.nested main.nested.func1 main.nested.func2.1 main.nested.func2.2 main.nested.func2 main.nested.func3]
// macro_file_line_func.gp 30 4
// 57 59 60
// macro_file_line_func.gp 69 main.main
// main.(*T).Ptr
// main.main.func1
// main.main.func2
// main.main.func3.1
