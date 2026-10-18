// phase: 05
// lsan: cycle

// Func values from interface methods inside interfaces and recursion (step 5.9): stored in an
// any and asserted back, passed to functions that call them on, returned, composed, and used as
// a method of an interface themselves.
package main

import "fmt"

type Op interface{ Apply(int) int }

type Add struct{ n int }

func (a Add) Apply(x int) int { return x + a.n }

type Mul struct{ n int }

func (m *Mul) Apply(x int) int { return x * m.n }

type Fn func(int) int

func (f Fn) Apply(x int) int { return f(x) }

func compose(ops ...Op) func(int) int {
	fs := make([]func(int) int, len(ops))
	for i, o := range ops {
		fs[i] = o.Apply
	}
	return func(x int) int {
		for _, f := range fs {
			x = f(x)
		}
		return x
	}
}

func fix(f func(func(int) int) func(int) int) func(int) int {
	var g func(int) int
	g = func(x int) int { return f(g)(x) }
	return g
}

func main() {
	var o Op = Add{3}
	var a any = o.Apply
	f, ok := a.(func(int) int)
	fmt.Println(ok, f(1))
	_, ok = a.(func() int)
	fmt.Println(ok)

	// A func value as an Op through a named func type, from a method value.
	var o2 Op = Fn(o.Apply)
	o = &Mul{5}
	fmt.Println(o2.Apply(10), o.Apply(10))
	o3 := Fn(o2.Apply)
	fmt.Println(o3.Apply(0))

	fmt.Println(compose(Add{1}, &Mul{2}, Fn(func(x int) int { return x - 3 }))(10))
	fmt.Println(compose()(7))

	// Recursion through a method value: factorial with the multiplier from an interface.
	var mul Op
	fact := fix(func(self func(int) int) func(int) int {
		return func(n int) int {
			if n <= 1 {
				return 1
			}
			mul = &Mul{n}
			return mul.Apply(self(n - 1))
		}
	})
	fmt.Println(fact(10))

	// Method values as map values and a dispatch table keyed by name.
	ops := map[string]Op{"add": Add{10}, "mul": &Mul{10}}
	table := map[string]func(int) int{}
	for name, op := range ops {
		table[name] = op.Apply
	}
	ops["add"] = Add{-1000}
	fmt.Println(table["add"](1), table["mul"](2), ops["add"].Apply(1))
	fmt.Println(len(table))
}

// Output:
// true 4
// false
// 13 50
// 3
// 19
// 7
// 512
// 11 20 -999
// 2
