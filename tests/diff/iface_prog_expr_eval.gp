// phase: 05

// An expression tree behind an interface: errors wrapped with %w, errors.Is/As, and a recover that turns a panic into an error.
package main

import (
	"errors"
	"fmt"
)

type Expr interface {
	Eval(env map[string]float64) (float64, error)
	String() string
}

type Num float64
type Var string
type Bin struct {
	Op   byte
	L, R Expr
}
type Neg struct{ X Expr }
type Call struct {
	Fn   string
	Args []Expr
}

var ErrDivZero = errors.New("division by zero")

type UnboundError struct{ Name string }

func (e *UnboundError) Error() string { return "unbound variable " + e.Name }

func (n Num) Eval(map[string]float64) (float64, error) { return float64(n), nil }
func (n Num) String() string                            { return fmt.Sprint(float64(n)) }
func (v Var) Eval(env map[string]float64) (float64, error) {
	if x, ok := env[string(v)]; ok {
		return x, nil
	}
	return 0, &UnboundError{string(v)}
}
func (v Var) String() string { return string(v) }
func (b *Bin) Eval(env map[string]float64) (float64, error) {
	l, err := b.L.Eval(env)
	if err != nil {
		return 0, fmt.Errorf("left of %c: %w", b.Op, err)
	}
	r, err := b.R.Eval(env)
	if err != nil {
		return 0, fmt.Errorf("right of %c: %w", b.Op, err)
	}
	switch b.Op {
	case '+':
		return l + r, nil
	case '-':
		return l - r, nil
	case '*':
		return l * r, nil
	case '/':
		if r == 0 {
			return 0, ErrDivZero
		}
		return l / r, nil
	}
	panic("bad op " + string(b.Op))
}
func (b *Bin) String() string { return fmt.Sprintf("(%v %c %v)", b.L, b.Op, b.R) }
func (n Neg) Eval(env map[string]float64) (float64, error) {
	x, err := n.X.Eval(env)
	return -x, err
}
func (n Neg) String() string { return fmt.Sprintf("-%v", n.X) }
func (c Call) Eval(env map[string]float64) (float64, error) {
	vals := make([]float64, 0, len(c.Args))
	for _, a := range c.Args {
		v, err := a.Eval(env)
		if err != nil {
			return 0, err
		}
		vals = append(vals, v)
	}
	switch c.Fn {
	case "max":
		m := vals[0]
		for _, v := range vals[1:] {
			if v > m {
				m = v
			}
		}
		return m, nil
	case "sum":
		s := 0.0
		for _, v := range vals {
			s += v
		}
		return s, nil
	}
	return 0, fmt.Errorf("unknown function %q", c.Fn)
}
func (c Call) String() string { return fmt.Sprintf("%s%v", c.Fn, c.Args) }

func safeEval(e Expr, env map[string]float64) (res float64, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("recovered: %v", r)
		}
	}()
	return e.Eval(env)
}

func main() {
	env := map[string]float64{"x": 3, "y": 4}
	exprs := []Expr{
		&Bin{'+', Num(1), &Bin{'*', Var("x"), Var("y")}},
		&Bin{'/', Num(1), &Bin{'-', Var("x"), Num(3)}},
		&Bin{'*', Var("z"), Num(2)},
		Neg{&Bin{'-', Var("x"), Var("y")}},
		Call{"max", []Expr{Num(1), Var("y"), Neg{Var("x")}}},
		Call{"sum", []Expr{Num(1.5), Num(2.5)}},
		Call{"nope", nil},
		&Bin{'%', Num(1), Num(2)},
	}
	for _, e := range exprs {
		v, err := safeEval(e, env)
		var ub *UnboundError
		switch {
		case errors.Is(err, ErrDivZero):
			fmt.Printf("%v => div by zero (%v)\n", e, err)
		case errors.As(err, &ub):
			fmt.Printf("%v => unbound %s (%v)\n", e, ub.Name, err)
		case err != nil:
			fmt.Printf("%v => error: %v\n", e, err)
		default:
			fmt.Printf("%v => %v\n", e, v)
		}
	}
}
