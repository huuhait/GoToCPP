// phase: 09

// The callee forms that F! accepts (docs/spec/language.md, "Must calls"): a method of a generic
// type, a must method called through an interface, a func-typed field, a method expression, a
// variadic function (also with ...), a call that gives all its results to another call, and fmt's
// Fprintf with the count it returns. The twin (testdata/must_twin/must_call_forms.go) writes the
// checks by hand.

package main

import (
	"errors"
	"fmt"
	"io"
	"strconv"
)

var ErrBad = errors.New("bad")

// A Starter is satisfied by a must method: it spells the real signature.
type Starter interface{ Start() error }

type Srv struct{ name string }

func (s *Srv) Start!() {
	n := strconv.Atoi(s.name)
	fmt.Println("started", n)
}

type Stack[T any] struct{ xs []T }

func (s *Stack[T]) Pop!() T {
	if len(s.xs) == 0 {
		return *new(T), fmt.Errorf("pop: %w", ErrBad)
	}
	x := s.xs[len(s.xs)-1]
	s.xs = s.xs[:len(s.xs)-1]
	return x
}

type Table struct{ lookup func(int) (int, error) }

func double(a int) (int, error) {
	if a < 0 {
		return 0, fmt.Errorf("double %d: %w", a, ErrBad)
	}
	return a * 2, nil
}

func count(xs ...int) (int, error) {
	if len(xs) == 0 {
		return 0, fmt.Errorf("count: %w", ErrBad)
	}
	return len(xs), nil
}

func pair() (int, int, error) { return 1, 2, nil }

func sum(a, b int) int { return a + b }

func run(name string, vals []int, n int) (total int, err error) {
	var st Starter = &Srv{name}
	st.Start!()

	s := &Stack[string]{xs: []string{"a", "b"}}
	fmt.Println(s.Pop!(), s.Pop!())

	t := Table{double}
	total += t.lookup!(n)

	start := (*Srv).Start
	start!(&Srv{"5"})

	total += count!(1, 2, 3) + count!(vals...)
	total += sum(pair!())

	w := fmt.Fprintf!(io.Discard, "hello %d", total)
	fmt.Println("wrote", w)
	fmt.Println(s.Pop!())
	return total, nil
}

func main() {
	fmt.Println(run("12", []int{1, 2}, 4))
	fmt.Println(run("x", []int{1}, 4))
	fmt.Println(run("12", nil, 4))
	fmt.Println(run("12", []int{1}, -4))
}

// Output:
// started 12
// b a
// started 5
// wrote 8
// 0 pop: bad
// 0 strconv.Atoi: parsing "x": invalid syntax
// started 12
// b a
// started 5
// 0 count: bad
// started 12
// b a
// 0 double -4: bad
