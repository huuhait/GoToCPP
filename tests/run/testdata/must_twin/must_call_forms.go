package main

import (
	"errors"
	"fmt"
	"io"
	"strconv"
)

var ErrBad = errors.New("bad")

type Starter interface{ Start() error }

type Srv struct{ name string }

func (s *Srv) Start() error {
	n, err := strconv.Atoi(s.name)
	if err != nil {
		return err
	}
	fmt.Println("started", n)
	return nil
}

type Stack[T any] struct{ xs []T }

func (s *Stack[T]) Pop() (T, error) {
	if len(s.xs) == 0 {
		return *new(T), fmt.Errorf("pop: %w", ErrBad)
	}
	x := s.xs[len(s.xs)-1]
	s.xs = s.xs[:len(s.xs)-1]
	return x, nil
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
	if err := st.Start(); err != nil {
		return 0, err
	}

	s := &Stack[string]{xs: []string{"a", "b"}}
	p1, err := s.Pop()
	if err != nil {
		return 0, err
	}
	p2, err := s.Pop()
	if err != nil {
		return 0, err
	}
	fmt.Println(p1, p2)

	t := Table{double}
	l, err := t.lookup(n)
	if err != nil {
		return 0, err
	}
	total += l

	start := (*Srv).Start
	if err := start(&Srv{"5"}); err != nil {
		return 0, err
	}

	c1, err := count(1, 2, 3)
	if err != nil {
		return 0, err
	}
	c2, err := count(vals...)
	if err != nil {
		return 0, err
	}
	total += c1 + c2
	a, b, err := pair()
	if err != nil {
		return 0, err
	}
	total += sum(a, b)

	w, err := fmt.Fprintf(io.Discard, "hello %d", total)
	if err != nil {
		return 0, err
	}
	fmt.Println("wrote", w)
	p3, err := s.Pop()
	if err != nil {
		return 0, err
	}
	fmt.Println(p3)
	return total, nil
}

func main() {
	fmt.Println(run("12", []int{1, 2}, 4))
	fmt.Println(run("x", []int{1}, 4))
	fmt.Println(run("12", nil, 4))
	fmt.Println(run("12", []int{1}, -4))
}
