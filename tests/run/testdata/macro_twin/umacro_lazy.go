// The hand-written Go twin of umacro_lazy.gp: each lazy parameter is a func() T, and each lazy
// argument a func literal.

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

var evals int

func next() int {
	evals++
	return evals
}

func name() string {
	evals++
	return fmt.Sprint("name", evals)
}

func Eval3(f func() int) int { return f() + f() + f() }

func Never(f func() int) int { return 7 }

func IfElse(c bool, a func() string, b func() string) string {
	if c {
		return a()
	}
	return b()
}

func Err(e func() error) error { return e() }

func Names(f func() []string) int { return len(f()) }

func Lookup(f func() map[string]int, k string) int { return f()[k] }

func Cfg(f func() Config) string {
	c := f()
	return fmt.Sprint(c.Name, ":", c.Port)
}

func Port(f func() *Config) int { return f().Port }

func Pred(f func() func(int) bool, n int) bool { return f()(n) }

func Lvl(f func() Level) Level { return f() + 1 }

func Arr(f func() [3]int) int { a := f(); return a[0] + a[2] }

func Store(f func() string) func() string {
	g := f
	return g
}

func Later(f func() string) {
	defer fmt.Println("deferred:", f())
	defer f()
	fmt.Println("in Later")
}

func Pass(use bool, f func() string) string {
	return IfElse(use, func() string { return f() }, func() string { return "not used" })
}

func Source(gp_text_f string, f func() string) string { return gp_text_f }

func Twice(f func() int) (int, int) { return f(), f() }

func main() {
	fmt.Println(Eval3(func() int { return next() }), evals)
	evals = 0
	fmt.Println(Never(func() int { return next() }), evals)
	fmt.Println(IfElse(true, func() string { return name() }, func() string { return name() }), IfElse(false, func() string { return name() }, func() string { return name() }), evals)
	fmt.Println(Err(func() error { return errors.New("e1") }), Err(func() error { return nil }))
	fmt.Println(Names(func() []string { return []string{"a", "b", "c"} }), Lookup(func() map[string]int { return map[string]int{"k": 4} }, "k"), Lookup(func() map[string]int { return nil }, "k"))
	fmt.Println(Cfg(func() Config { return Config{"srv", 80} }), Port(func() *Config { return &Config{Port: 8080} }), Lvl(func() Level { return Level(41) }))
	fmt.Println(Pred(func() func(int) bool { return func(n int) bool { return n > 2 } }, 3), Arr(func() [3]int { return [3]int{1, 2, 3} }))

	x := 1
	g := Store(func() string { return fmt.Sprint("x is ", x) })
	x = 2
	fmt.Println(g())
	x = 3
	fmt.Println(g())

	Later(func() string { return name() })
	evals = 0
	fmt.Println(Pass(true, func() string { return name() }), Pass(false, func() string { return name() }), evals)
	sa, sb := "x", "y"
	fmt.Println(Source("sa+sb", func() string { return sa + sb }), Source(`"s"`, func() string { return "s" }))
	c, d := Twice(func() int { return next() })
	fmt.Println(c, d)
}
