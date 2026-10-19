// phase: 05

// A named result hidden by an inner variable, with results of struct, interface, slice, map and
// pointer types, swapped results, results that the deferred call changes after the return, and a
// panic that a deferred call recovers.
package main

import (
	"errors"
	"fmt"
)

type P struct{ X, Y int }

type S interface{ Name() string }

type N string

func (n N) Name() string { return string(n) }

func structs(ps []P) (p P, ok bool) {
	defer func() { p.Y += 100 }()
	for _, p := range ps {
		if p.X > 1 {
			return P{p.X, p.Y * 2}, true
		}
	}
	return P{}, false
}

func ifaces(names []string) (s S, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("recovered: %v", r)
		}
	}()
	for _, s := range names {
		if s == "boom" {
			panic("boom!")
		}
		if s != "" {
			return N(s), nil
		}
	}
	return nil, errors.New("empty")
}

func slices(n int) (xs []int, m map[string]int) {
	defer func() { xs = append(xs, -1) }()
	for xs := 0; xs < n; xs++ {
		if xs == 2 {
			return []int{xs, xs * xs}, map[string]int{"n": n}
		}
	}
	return nil, nil
}

func pointers() (p *int, q *P) {
	defer func() { *p += 1 }()
	for p := 0; p < 1; p++ {
		x := 41
		return &x, &P{1, 2}
	}
	return nil, nil
}

func swapped() (i, j int) {
	i, j = 1, 2
	defer func() {}()
	return j, i
}

func swapShadow() (i, j int) {
	i, j = 1, 2
	defer func() { i, j = i*10, j*10 }()
	for i := 7; i < 8; i++ {
		return j, i
	}
	return 0, 0
}

func swapShadowBoth() (i, j string) {
	defer func() { i += "!" }()
	for i, j := "a", "b"; i != ""; i = "" {
		return j, i
	}
	return "", ""
}

func panics(d int) (i int, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%v", r)
		}
	}()
	for i := 5; i < 6; i++ {
		if d == 0 {
			return i / d, nil
		}
		return i / d, nil
	}
	return -1, nil
}

func blankResults() (_ int, i int, _ string) {
	defer func() {}()
	for i := 3; i < 4; i++ {
		return i, i * 2, "b"
	}
	return
}

func variadic(base int, xs ...int) (i int, total []int) {
	defer func() { total = append(total, 0) }()
	for i, x := range xs {
		if x < 0 {
			return i + base, xs[:i]
		}
	}
	return len(xs), xs
}

func unnamedInside() (int, string) {
	f := func() (i int, s string) {
		defer func() {}()
		for i, s := range []string{"p", "q"} {
			if i == 1 {
				return i + 10, s
			}
		}
		return
	}
	i, s := f()
	return i * 2, s + "?"
}

func outerNamed() (i int, f func() int) {
	defer func() {}()
	i = 5
	f = func() int {
		defer func() {}()
		for i := 0; i < 3; i++ {
			if i == 2 {
				return i
			}
		}
		return -1
	}
	return i + 1, f
}

func main() {
	fmt.Println(structs([]P{{1, 2}, {3, 4}}))
	fmt.Println(structs(nil))
	s, err := ifaces([]string{"", "ab"})
	fmt.Println(s.Name(), err)
	s, err = ifaces([]string{"", "boom"})
	fmt.Println(s, err)
	_, err = ifaces(nil)
	fmt.Println(err)
	fmt.Println(slices(5))
	fmt.Println(slices(1))
	p, q := pointers()
	fmt.Println(*p, *q)
	fmt.Println(swapped())
	fmt.Println(swapShadow())
	fmt.Println(swapShadowBoth())
	fmt.Println(panics(1))
	fmt.Println(panics(0))
	fmt.Println(blankResults())
	fmt.Println(variadic(100, 1, 2, -3, 4))
	fmt.Println(variadic(100, 1, 2))
	fmt.Println(unnamedInside())
	i, f := outerNamed()
	fmt.Println(i, f())
}

// Output:
// {3 108} true
// {0 100} false
// ab <nil>
// <nil> recovered: boom!
// empty
// [2 4 -1] map[n:5]
// [-1] map[]
// 42 {1 2}
// 2 1
// 20 70
// b! a
// 5 <nil>
// 0 runtime error: integer divide by zero
// 3 6 b
// 102 [1 2 0]
// 2 [1 2 0]
// 22 q?
// 6 2
