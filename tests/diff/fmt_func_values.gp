// phase: 05

// The functions of fmt as values: variables, parameters, map values and package variables; the
// variadic ...any spreads into the operands.
package main

import (
	"errors"
	"fmt"
)

var _ = fmt.Sprint

func apply(f func(a ...any) (int, error), xs ...any) {
	n, err := f(xs...)
	fmt.Println("n =", n, err)
}

func main() {
	h := fmt.Println
	h("x", 1, nil)
	g := fmt.Sprintf
	fmt.Println(g("%d-%s", 3, "a"), g("plain"))
	apply(fmt.Println, "via", "param")
	apply(fmt.Print, "p1", "p2\n")
	e := fmt.Errorf
	err := e("wrap %w", errors.New("in"))
	fmt.Println(err, errors.Unwrap(err))
	sp := fmt.Sprint
	sl := fmt.Sprintln
	fmt.Print(sp("a", 1, 2, "b"), "|", sl("z", 3))
	pf := fmt.Printf
	pf("%v %T\n", []int{1}, 2.5)
	pf("no args\n")
	fns := map[string]func(string, ...any) string{"s": fmt.Sprintf}
	fmt.Println(fns["s"]("%03d", 7))
	ap := fmt.Appendf
	b := ap([]byte("x="), "%d", 5)
	fmt.Println(string(b), string(fmt.Append(nil, "q", 1)))
	var nilf func(...any) string = fmt.Sprint
	fmt.Println(nilf != nil)
}
