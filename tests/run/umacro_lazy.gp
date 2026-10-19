// phase: 05

// Lazy parameters (step 10.9): the argument is not evaluated at the use; the macro runs it, zero,
// one or several times, when it calls the parameter. The type after lazy is written at every use,
// so it is a predeclared type, a type of the macro's package that the use can name, or a type of a
// package that both files import. The twin (testdata/macro_twin/umacro_lazy.go) is the same
// program with each lazy argument written as a func literal.

package main

import (
	"errors"
	"fmt"
)

type Level int

type Config struct {
	Name string
	Port int
}

// evals counts the evaluations of the lazy arguments.
var evals int

func next() int {
	evals++
	return evals
}

func name() string {
	evals++
	return fmt.Sprint("name", evals)
}

macro Eval3(f lazy int) int { return f() + f() + f() }

macro Never(f lazy int) int { return 7 }

macro IfElse(c bool, a lazy string, b lazy string) string {
	if c {
		return a()
	}
	return b()
}

macro Err(e lazy error) error { return e() }

macro Names(f lazy []string) int { return len(f()) }

macro Lookup(f lazy map[string]int, k string) int { return f()[k] }

macro Cfg(f lazy Config) string {
	c := f()
	return fmt.Sprint(c.Name, ":", c.Port)
}

macro Port(f lazy *Config) int { return f().Port }

macro Pred(f lazy func(int) bool, n int) bool { return f()(n) }

macro Lvl(f lazy Level) Level { return f() + 1 }

macro Arr(f lazy [3]int) int { a := f(); return a[0] + a[2] }

// Store keeps the lazy argument: it runs later, when the func is called.
macro Store(f lazy string) func() string {
	g := f
	return g
}

macro Later(f lazy string) {
	defer fmt.Println("deferred:", f())
	defer f()
	fmt.Println("in Later")
}

// Pass hands its lazy parameter to another macro's lazy parameter.
macro Pass(use bool, f lazy string) string {
	return #IfElse(use, f(), "not used")
}

macro Source(f lazy string) string { return #text(f) }

macro Twice(f lazy int) (int, int) { return f(), f() }

func main() {
	fmt.Println(#Eval3(next()), evals)
	evals = 0
	fmt.Println(#Never(next()), evals)
	fmt.Println(#IfElse(true, name(), name()), #IfElse(false, name(), name()), evals)
	fmt.Println(#Err(errors.New("e1")), #Err(nil))
	fmt.Println(#Names([]string{"a", "b", "c"}), #Lookup(map[string]int{"k": 4}, "k"), #Lookup(nil, "k"))
	fmt.Println(#Cfg(Config{"srv", 80}), #Port(&Config{Port: 8080}), #Lvl(Level(41)))
	fmt.Println(#Pred(func(n int) bool { return n > 2 }, 3), #Arr([3]int{1, 2, 3}))

	// A lazy argument is a closure: it sees the variable as it is when the macro calls it.
	x := 1
	g := #Store(fmt.Sprint("x is ", x))
	x = 2
	fmt.Println(g())
	x = 3
	fmt.Println(g())

	#Later(name())
	evals = 0
	fmt.Println(#Pass(true, name()), #Pass(false, name()), evals)
	sa, sb := "x", "y"
	fmt.Println(#Source(sa+sb), #Source("s"))
	c, d := #Twice(next())
	fmt.Println(c, d)
}

// Output:
// 6 3
// 7 0
// name1 name2 2
// e1 <nil>
// 3 4 0
// srv:80 8080 42
// true 4
// x is 2
// x is 3
// in Later
// deferred: name3
// name1 not used 1
// sa+sb "s"
// 2 3
