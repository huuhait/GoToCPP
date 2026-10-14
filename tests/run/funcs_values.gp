// phase: 03

// Func values: functions of the package as values, nil and == nil, func values passed,
// returned and stored in variables, slices, maps and struct fields, funcs with several results,
// and funcs that take and return funcs.
package main

import "fmt"

func add(a, b int) int { return a + b }
func sub(a, b int) int { return a - b }

func divmod(a, b int) (int, int) { return a / b, a % b }

func pick(name string) func(int, int) int {
	switch name {
	case "add":
		return add
	case "sub":
		return sub
	}
	return nil
}

func twice(f func(int) int) func(int) int {
	return func(x int) int { return f(f(x)) }
}

func inc(x int) int { return x + 1 }

type op struct {
	name string
	fn   func(int, int) int
}

func run(ops []op, a, b int) {
	for _, o := range ops {
		if o.fn == nil {
			fmt.Println(o.name, "missing")
			continue
		}
		fmt.Println(o.name, o.fn(a, b))
	}
}

func main() {
	f := add
	fmt.Println(f(2, 3), f != nil)
	f = sub
	fmt.Println(f(2, 3))

	var g func(int, int) int
	fmt.Println(g == nil, nil == g)
	g = pick("add")
	fmt.Println(g(10, 20), pick("mul") == nil)

	dm := divmod
	q, r := dm(17, 5)
	fmt.Println(q, r)
	fmt.Println(dm(9, 4))

	fmt.Println(twice(inc)(5), twice(twice(inc))(0))

	fs := []func(int, int) int{add, sub, nil}
	for i, h := range fs {
		if h != nil {
			fmt.Println(i, h(7, 2))
		}
	}

	table := map[string]func(int, int) int{"add": add, "sub": sub}
	if h, ok := table["sub"]; ok {
		fmt.Println("sub", h(1, 10))
	}
	if _, ok := table["mul"]; !ok {
		fmt.Println("no mul", table["mul"] == nil)
	}
	table["mul"] = func(a, b int) int { return a * b }
	fmt.Println(table["mul"](6, 7), len(table))

	run([]op{{"add", add}, {"none", nil}, {"pow", func(a, b int) int {
		p := 1
		for range b {
			p *= a
		}
		return p
	}}}, 2, 10)

	s := op{name: "s"}
	s.fn = sub
	p := &s
	fmt.Println(p.fn(5, 3), s.fn(3, 5))
	p.fn = nil
	fmt.Println(s.fn == nil)
}

// Output:
// 5 true
// -1
// true true
// 30 true
// 3 2
// 2 1
// 7 4
// 0 9
// 1 5
// sub -9
// no mul true
// 42 3
// add 12
// none missing
// pow 1024
// 2 -2
// true
