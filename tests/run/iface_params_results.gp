// phase: 05

// Interface parameters and results, variadic ...any and ...Shape, multiple results, nil
// arguments (step 5.4).
package main

import "fmt"

type Stringer interface{ String() string }

type A int
type B string

func (a A) String() string { return fmt.Sprint("A", int(a)) }
func (b B) String() string { return "B" + string(b) }

func join(sep string, xs ...Stringer) string {
	s := ""
	for i, x := range xs {
		if i > 0 {
			s += sep
		}
		if x == nil {
			s += "<nil>"
		} else {
			s += x.String()
		}
	}
	return s
}

func count(xs ...any) (n, strs int) {
	for _, x := range xs {
		n++
		if _, ok := x.(string); ok {
			strs++
		}
	}
	return
}

func first(xs ...any) any {
	if len(xs) == 0 {
		return nil
	}
	return xs[0]
}

func pick(b bool) (Stringer, any) {
	if b {
		return A(1), 1
	}
	return B("x"), "x"
}

func pass(x any) any { return x }

func main() {
	fmt.Println(join(",", A(1), B("b"), nil, A(3)))
	fmt.Println(join(";"))
	list := []Stringer{A(5), B("z")}
	fmt.Println(join("-", list...))
	fmt.Println(count(1, "a", nil, 2.5, "b"))
	fmt.Println(count())
	args := []any{"x", 1, "y"}
	fmt.Println(count(args...))
	fmt.Println(first() == nil, first(nil) == nil, first(0) == nil, first(1, 2).(int))
	s, a := pick(true)
	fmt.Println(s.String(), a.(int))
	s, a = pick(false)
	fmt.Println(s.String(), a.(string))
	fmt.Println(pass(pass(pass(7))).(int), pass(nil) == nil)
}

// Output:
// A1,Bb,<nil>,A3
//
// A5-Bz
// 5 2
// 0 0
// 3 2
// true true false 1
// A1 1
// Bx x
// 7 true
